package relocation

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"portscoop/internal/environment"
	"portscoop/internal/metadata"
)

func digest(data []byte) string { hash := sha256.Sum256(data); return hex.EncodeToString(hash[:]) }

func validateShim(data []byte, root string) error {
	text := strings.TrimPrefix(string(data), "\ufeff")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var target string
	switch {
	case len(lines) >= 1 && strings.HasPrefix(lines[0], `path = "`) && strings.HasSuffix(lines[0], `"`):
		target = strings.TrimSuffix(strings.TrimPrefix(lines[0], `path = "`), `"`)
		if len(lines) > 2 || (len(lines) == 2 && !strings.HasPrefix(lines[1], "args = ")) {
			return fmt.Errorf("unknown EXE shim metadata")
		}
	case len(lines) >= 2 && strings.HasPrefix(lines[0], "@rem "):
		target = strings.TrimPrefix(lines[0], "@rem ")
		ext := strings.ToLower(filepath.Ext(target))
		valid := (len(lines) == 2 && ((ext == ".cmd" || ext == ".bat") && strings.HasPrefix(lines[1], `@"`+target+`" `) || ext == ".py" && strings.HasPrefix(lines[1], `@python "`+target+`" `))) ||
			(ext == ".ps1" && len(lines) == 8 && lines[1] == "@echo off" && lines[2] == "where /q pwsh.exe") ||
			(ext == ".jar" && len(lines) == 4 && strings.HasPrefix(lines[1], "@pushd ")) ||
			(len(lines) == 11 && lines[1] == "@echo off" && lines[2] == `bash -c "command -v wslpath >/dev/null"`)
		if !valid {
			return fmt.Errorf("unknown CMD shim format")
		}
	case len(lines) >= 3 && lines[0] == "#!/bin/sh" && strings.HasPrefix(lines[1], "# "):
		target = strings.TrimPrefix(lines[1], "# ")
		valid := (len(lines) == 3 && (strings.HasPrefix(lines[2], "MSYS2_ARG_CONV_EXCL=/C cmd.exe /C ") || strings.HasPrefix(lines[2], "python.exe "))) ||
			(len(lines) == 7 && lines[2] == "if command -v pwsh.exe > /dev/null 2>&1; then") ||
			(len(lines) == 8 && lines[2] == "if [ $WSL_INTEROP ]") || (len(lines) == 9 && lines[2] == "if [ $WSL_INTEROP ]")
		if !valid {
			return fmt.Errorf("unknown shell shim format")
		}
	case len(lines) == 4 && strings.HasPrefix(lines[0], "# ") && strings.HasPrefix(lines[1], "$path = "):
		target = strings.TrimPrefix(lines[0], "# ")
		if !strings.HasPrefix(lines[2], "if ($MyInvocation.ExpectingInput)") || lines[3] != "exit $LASTEXITCODE" {
			return fmt.Errorf("unknown PowerShell shim format")
		}
	default:
		return fmt.Errorf("unsupported shim format")
	}
	if !filepath.IsAbs(target) || !within(root, target) {
		return fmt.Errorf("shim target escapes prepared root")
	}
	return nil
}

func normalizedPE(data []byte) ([]byte, error) {
	copy := append([]byte(nil), data...)
	if len(copy) < 64 || string(copy[:2]) != "MZ" {
		return nil, fmt.Errorf("unknown binary shim")
	}
	header := int(binary.LittleEndian.Uint32(copy[60:64]))
	subsystem := header + 24 + 68
	if header < 0 || subsystem+2 > len(copy) || string(copy[header:header+4]) != "PE\x00\x00" {
		return nil, fmt.Errorf("unknown binary shim header")
	}
	copy[subsystem], copy[subsystem+1] = 0, 0
	return copy, nil
}

func validateBinaryShim(paths environment.Paths, data []byte) error {
	engine := filepath.Dir(filepath.Dir(paths.Engine))
	expected, err := os.ReadFile(filepath.Join(engine, "supporting", "shims", "kiennq", "shim.exe"))
	if err != nil {
		return err
	}
	actual, err := normalizedPE(data)
	if err != nil {
		return err
	}
	want, err := normalizedPE(expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, want) {
		return fmt.Errorf("unknown binary shim implementation")
	}
	return nil
}

func persistDeclarations(data []byte) ([][2]string, error) {
	var manifest struct {
		Persist json.RawMessage `json:"persist"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if len(manifest.Persist) == 0 {
		return nil, nil
	}
	var single string
	var entries []json.RawMessage
	if json.Unmarshal(manifest.Persist, &single) == nil {
		entry, _ := json.Marshal(single)
		entries = []json.RawMessage{entry}
	} else if err := json.Unmarshal(manifest.Persist, &entries); err != nil {
		return nil, err
	}
	var result [][2]string
	for _, raw := range entries {
		var source, target string
		if json.Unmarshal(raw, &source) == nil {
			target = source
		} else {
			var tuple []string
			if err := json.Unmarshal(raw, &tuple); err != nil || len(tuple) != 2 {
				return nil, fmt.Errorf("unsupported persist metadata")
			}
			source, target = tuple[0], tuple[1]
		}
		if !metadata.RelativePath(source) || !metadata.RelativePath(target) {
			return nil, fmt.Errorf("persist metadata escapes root")
		}
		result = append(result, [2]string{source, target})
	}
	return result, nil
}

func expectedPersist(paths environment.Paths) (map[string]string, error) {
	result := map[string]string{}
	apps, err := os.ReadDir(paths.Apps)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		if app.Name() == "scoop" {
			continue
		}
		if !app.IsDir() || !metadata.RelativePath(app.Name()) {
			return nil, fmt.Errorf("unsupported app directory")
		}
		versions, err := os.ReadDir(filepath.Join(paths.Apps, app.Name()))
		if err != nil {
			return nil, err
		}
		for _, version := range versions {
			if version.Name() == "current" {
				continue
			}
			if !version.IsDir() || !metadata.RelativePath(version.Name()) {
				return nil, fmt.Errorf("unsupported app version")
			}
			base := filepath.Join(paths.Apps, app.Name(), version.Name())
			if err := containedParent(paths.Root, filepath.Join(base, "scoop-manifest.json")); err != nil {
				return nil, err
			}
			data, err := os.ReadFile(filepath.Join(base, "scoop-manifest.json"))
			if os.IsNotExist(err) {
				data, err = os.ReadFile(filepath.Join(base, "manifest.json"))
			}
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			declarations, err := persistDeclarations(data)
			if err != nil {
				return nil, err
			}
			for _, entry := range declarations {
				source := filepath.Join(base, entry[0])
				target := filepath.Join(paths.Persist, app.Name(), entry[1])
				relative, _ := filepath.Rel(paths.Root, source)
				targetRelative, _ := filepath.Rel(paths.Root, target)
				result[filepath.ToSlash(relative)] = filepath.ToSlash(targetRelative)
			}
		}
	}
	return result, nil
}

func inventory(paths environment.Paths) ([]Artifact, error) {
	var artifacts []Artifact
	entries, err := os.ReadDir(paths.Shims)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		path := filepath.Join(paths.Shims, entry.Name())
		if err := containedParent(paths.Root, path); err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("unsupported shim path: %s", entry.Name())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		a := Artifact{Path: "scoop/shims/" + entry.Name(), Kind: "shim", Content: data}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".exe") {
			if err := validateBinaryShim(paths, data); err != nil {
				return nil, err
			}
			a.Kind = "binary"
			a.Content = nil
			a.Digest = digest(data)
		} else if err := validateShim(data, paths.Root); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		artifacts = append(artifacts, a)
	}
	persist, err := expectedPersist(paths)
	if err != nil {
		return nil, err
	}
	seenLinks := map[string]bool{}
	if _, err := os.Stat(paths.Apps); err == nil {
		err = filepath.WalkDir(paths.Apps, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == filepath.Join(paths.Apps, "scoop") {
				return filepath.SkipDir
			}
			if entry.Type()&(os.ModeSymlink|os.ModeIrregular) == 0 {
				return nil
			}
			relative, _ := filepath.Rel(paths.Root, path)
			relative = filepath.ToSlash(relative)
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			if !within(paths.Root, target) {
				return fmt.Errorf("managed link escapes root: %s", relative)
			}
			targetRelative, _ := filepath.Rel(paths.Root, target)
			targetRelative = filepath.ToSlash(targetRelative)
			a := Artifact{Path: relative, Kind: "junction", Target: targetRelative}
			if _, err := artifactPath(paths, a); err != nil {
				return err
			}
			if !strings.HasSuffix(relative, "/current") && persist[relative] != targetRelative {
				return fmt.Errorf("unowned package link: %s", relative)
			}
			resolved, err := filepath.EvalSymlinks(target)
			if err != nil || !within(paths.Root, resolved) {
				return fmt.Errorf("invalid managed link target: %s", relative)
			}
			artifacts = append(artifacts, a)
			seenLinks[relative] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for source, target := range persist {
		if seenLinks[source] {
			continue
		}
		sourcePath, targetPath := filepath.Join(paths.Root, filepath.FromSlash(source)), filepath.Join(paths.Root, filepath.FromSlash(target))
		if err := containedParent(paths.Root, sourcePath); err != nil {
			return nil, err
		}
		if err := containedParent(paths.Root, targetPath); err != nil {
			return nil, err
		}
		left, err := os.Stat(sourcePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		right, err := os.Stat(targetPath)
		if err != nil {
			return nil, err
		}
		if left.IsDir() || right.IsDir() || !os.SameFile(left, right) {
			return nil, fmt.Errorf("unsupported persisted file link: %s", source)
		}
		artifacts = append(artifacts, Artifact{Path: source, Kind: "hardlink", Target: target})
	}
	environmentPath := filepath.Join(paths.State, "environment.json")
	if err := containedParent(paths.Root, environmentPath); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(environmentPath); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("portable environment state is a reparse point")
		}
		data, err := os.ReadFile(environmentPath)
		if err != nil {
			return nil, err
		}
		if _, err = environmentContent(data, paths.Root, paths.Root); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{Path: ".portscoop/state/environment.json", Kind: "environment", Content: data})
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func environmentContent(data []byte, from, to string) ([]byte, error) {
	var state struct {
		SchemaVersion int               `json:"schema_version"`
		Values        map[string]string `json:"values"`
	}
	if err := metadata.Decode(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &state); err != nil {
		return nil, err
	}
	if state.SchemaVersion != 1 || state.Values == nil {
		return nil, fmt.Errorf("unsupported portable environment state")
	}
	reserved := map[string]bool{"SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true, "PSMODULEPATH": true, "USERPROFILE": true, "HOME": true, "APPDATA": true, "LOCALAPPDATA": true, "PROGRAMDATA": true, "TEMP": true, "TMP": true, "XDG_CONFIG_HOME": true}
	for name, value := range state.Values {
		upper := strings.ToUpper(name)
		if reserved[upper] || strings.HasPrefix(upper, "SCOOP") || strings.HasPrefix(upper, "PORTSCOOP") || strings.HasPrefix(upper, "GIT_") {
			return nil, fmt.Errorf("reserved variable in portable state: %s", name)
		}
		if upper == "PATH" {
			for _, entry := range strings.Split(value, ";") {
				if entry != "" && !within(from, entry) && !within(to, entry) {
					return nil, fmt.Errorf("portable PATH contains a foreign entry")
				}
			}
		}
		state.Values[name] = relocateText(value, from, to)
	}
	return json.MarshalIndent(state, "", "  ")
}
