package relocation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"portscoop/internal/environment"
	"portscoop/internal/metadata"
	"portscoop/internal/shimargs"
)

// ShimDescriptor describes original targets, never an executable wrapper command.
// Unsupported is explicit even for wrappers recognized by the ownership scanner.
type ShimDescriptor struct {
	Name, Target, Handler, Unsupported string
	Arguments                          []string
}

var shimName = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.+-]*$`)

func ValidateShimName(name string) error {
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	device := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CLOCK$" || base == "CONIN$" || base == "CONOUT$"
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
		device = true
	}
	if !shimName.MatchString(name) || strings.HasSuffix(name, ".") || device {
		return fmt.Errorf("invalid logical shim name %q: paths and device names are not permitted", name)
	}
	return nil
}

// OwnedShims requires a prepared inventory, validates it without adopting edits,
// and groups the generated representations of each logical alias.
// The caller must hold the Store operation lock throughout resolution and start.
func (m Manager) OwnedShims(paths environment.Paths) ([]ShimDescriptor, error) {
	if err := m.Prepare(paths); err != nil {
		return nil, err
	}
	artifacts, err := inventory(paths)
	if err != nil {
		return nil, err
	}
	groups := map[string][]Artifact{}
	var order []string
	for _, a := range artifacts {
		if a.Kind != "shim" && a.Kind != "binary" {
			continue
		}
		name := filepath.Base(a.Path)
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".exe" || ext == ".shim" || ext == ".cmd" || ext == ".ps1" {
			name = strings.TrimSuffix(name, filepath.Ext(name))
		}
		if err := ValidateShimName(name); err != nil {
			return nil, err
		}
		key := strings.ToLower(name)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], a)
	}
	var result []ShimDescriptor
	for _, name := range order {
		d := ShimDescriptor{Name: name}
		binary, nativeMetadata := false, false
		for _, a := range groups[name] {
			if a.Kind == "binary" {
				binary = true
				continue
			}
			target, unsupported := descriptorTarget(a.Content)
			if target == "" {
				return nil, fmt.Errorf("shim %s has no qualified target metadata", name)
			}
			if d.Target != "" && !strings.EqualFold(d.Target, target) {
				return nil, fmt.Errorf("conflicting shim definitions for %s", name)
			}
			d.Target = target
			if unsupported != "" {
				d.Unsupported = unsupported
			}
			if strings.EqualFold(filepath.Ext(a.Path), ".shim") {
				nativeMetadata = true
				text := strings.TrimPrefix(strings.ReplaceAll(string(a.Content), "\r\n", "\n"), "\ufeff")
				lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
				if len(lines) == 2 && strings.HasPrefix(lines[1], "args = ") {
					d.Arguments, err = shimargs.Parse(strings.TrimPrefix(lines[1], "args = "))
					if err != nil {
						return nil, fmt.Errorf("shim %s: %w", name, err)
					}
				}
			}
		}
		if binary != nativeMetadata {
			return nil, fmt.Errorf("incomplete native shim family %s", name)
		}
		switch strings.ToLower(filepath.Ext(d.Target)) {
		case ".exe", ".com":
			if !binary {
				return nil, fmt.Errorf("native shim %s lacks its owned executable", name)
			}
			d.Handler = "native"
		case ".ps1":
			d.Handler = "powershell"
		case ".cmd", ".bat":
			d.Handler = "batch"
		default:
			d.Handler = "unsupported"
			d.Unsupported = "no qualified Store-bound runtime handler for " + filepath.Ext(d.Target)
		}
		result = append(result, d)
	}
	return result, nil
}

func descriptorTarget(data []byte) (string, string) {
	text := strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\ufeff")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	var target string
	unsupported := ""
	switch {
	case strings.HasPrefix(lines[0], `path = "`):
		target = strings.TrimSuffix(strings.TrimPrefix(lines[0], `path = "`), `"`)
		if len(lines) > 2 || (len(lines) == 2 && !strings.HasPrefix(lines[1], "args = ")) {
			unsupported = "nonstandard native shim arguments"
		}
	case strings.HasPrefix(lines[0], "@rem "):
		target = strings.TrimPrefix(lines[0], "@rem ")
		switch strings.ToLower(filepath.Ext(target)) {
		case ".cmd", ".bat":
			if len(lines) != 2 || lines[1] != `@"`+target+`"  %*` {
				unsupported = "fixed or nonstandard batch shim arguments are not qualified"
			}
		case ".ps1":
			if len(lines) != 8 || lines[4] != `    pwsh -noprofile -ex unrestricted -file "`+target+`"  %*` || lines[6] != `    powershell -noprofile -ex unrestricted -file "`+target+`"  %*` {
				unsupported = "fixed or nonstandard PowerShell shim arguments are not qualified"
			}
		}
	case lines[0] == "#!/bin/sh" && len(lines) > 1:
		target = strings.TrimPrefix(lines[1], "# ")
		switch strings.ToLower(filepath.Ext(target)) {
		case ".cmd", ".bat":
			if len(lines) != 3 || lines[2] != `MSYS2_ARG_CONV_EXCL=/C cmd.exe /C "`+target+`"  "$@"` {
				unsupported = "fixed or nonstandard shell shim arguments are not qualified"
			}
		case ".ps1":
			if len(lines) != 7 || lines[3] != `    pwsh.exe -noprofile -ex unrestricted -file "`+target+`"  "$@"` || lines[5] != `    powershell.exe -noprofile -ex unrestricted -file "`+target+`"  "$@"` {
				unsupported = "fixed or nonstandard PowerShell shim arguments are not qualified"
			}
		}
	case strings.HasPrefix(lines[0], "# "):
		target = strings.TrimPrefix(lines[0], "# ")
		if len(lines) != 4 || lines[2] != "if ($MyInvocation.ExpectingInput) { $input | & $path  @args } else { & $path  @args }" {
			unsupported = "fixed or nonstandard PowerShell shim arguments are not qualified"
		}
	}
	return target, unsupported
}

// ResolveOwnedShim never probes host PATH or another Store. Call under the lock.
func (m Manager) ResolveOwnedShim(paths environment.Paths, name string) (ShimDescriptor, error) {
	if err := ValidateShimName(name); err != nil {
		return ShimDescriptor{}, err
	}
	shims, err := m.OwnedShims(paths)
	if err != nil {
		return ShimDescriptor{}, err
	}
	for _, shim := range shims {
		if !strings.EqualFold(shim.Name, name) {
			continue
		}
		if shim.Unsupported != "" {
			return shim, fmt.Errorf("shim %s: %s", name, shim.Unsupported)
		}
		target, err := canonicalTarget(shim.Target)
		if err != nil {
			return shim, fmt.Errorf("shim %s target is unavailable: %w", name, err)
		}
		// A target belongs to one package and version. Persist links may exist
		// elsewhere, but executable targets cannot redirect outside that package.
		rel, err := filepath.Rel(paths.Apps, shim.Target)
		parts := strings.Split(rel, string(filepath.Separator))
		if err != nil || !metadata.RelativePath(rel) || len(parts) < 3 || strings.EqualFold(parts[0], "scoop") || !within(filepath.Join(paths.Apps, parts[0]), target) {
			return shim, fmt.Errorf("shim %s target escapes its package: %s", name, shim.Target)
		}
		info, err := os.Stat(target)
		if err != nil || !info.Mode().IsRegular() {
			return shim, fmt.Errorf("shim %s target is not a regular package file: %s", name, shim.Target)
		}
		shim.Target = target
		return shim, nil
	}
	return ShimDescriptor{}, fmt.Errorf("shim %q is not installed in the selected Store", name)
}
