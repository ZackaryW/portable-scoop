package scoop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"portscoop/internal/metadata"
)

const PolicyVersion = "portable-zip-v1"

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var hashPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
var versionPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
var variablePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type Operation struct {
	Name      string
	Arguments []string
	Packages  []string
	Preflight bool
}

// PackageName accepts a name or bucket/name, never a path, URL, option, or
// generated @version manifest. Pinning is supplied by the selected bucket.
func PackageName(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !namePattern.MatchString(part) || !metadata.RelativePath(part) {
			return false
		}
	}
	return !strings.EqualFold(parts[len(parts)-1], "scoop")
}

// Classify validates the complete public backend command before dispatch. It
// intentionally does not offer a raw-command escape hatch.
func Classify(args []string) (Operation, error) {
	if len(args) == 0 {
		return Operation{}, fmt.Errorf("a package command is required")
	}
	op := Operation{Name: args[0], Arguments: append([]string(nil), args...)}
	rest := args[1:]
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return op, fmt.Errorf("invalid command argument")
		}
	}
	switch op.Name {
	case "install", "uninstall", "update", "reset", "cleanup":
		op.Preflight = true
		for _, arg := range rest {
			if strings.HasPrefix(arg, "-") {
				allowed := (arg == "--no-cache" && (op.Name == "install" || op.Name == "update")) || (arg == "--force" && op.Name == "update")
				if !allowed {
					return op, fmt.Errorf("option %q is unsupported by %s", arg, PolicyVersion)
				}
				continue
			}
			if arg == "*" && (op.Name == "update" || op.Name == "cleanup") {
				op.Packages = append(op.Packages, arg)
				continue
			}
			if !PackageName(arg) {
				return op, fmt.Errorf("invalid or unsupported package reference %q", arg)
			}
			op.Packages = append(op.Packages, arg)
		}
		if len(op.Packages) == 0 {
			return op, fmt.Errorf("%s requires a package; engine self-update is managed by Portscoop releases", op.Name)
		}
	case "list", "search":
		if len(rest) > 1 || (len(rest) == 1 && strings.HasPrefix(rest[0], "-")) {
			return op, fmt.Errorf("%s accepts one optional filter", op.Name)
		}
	case "status":
		if len(rest) != 0 {
			return op, fmt.Errorf("status takes no package arguments")
		}
	case "config":
		if len(rest) > 1 || (len(rest) == 1 && (!variablePattern.MatchString(rest[0]) || rest[0] == "rm")) {
			return op, fmt.Errorf("configuration is inspection-only")
		}
	case "bucket":
		if len(rest) == 0 {
			return op, fmt.Errorf("bucket requires list, add, or rm")
		}
		switch rest[0] {
		case "list":
			if len(rest) != 1 {
				return op, fmt.Errorf("bucket list takes no further arguments")
			}
		case "add":
			if len(rest) < 2 || len(rest) > 3 || !PackageName(rest[1]) || strings.Contains(rest[1], "/") {
				return op, fmt.Errorf("bucket add requires a name and optional repository")
			}
			if len(rest) == 3 {
				u, err := url.Parse(rest[2])
				if !filepath.IsAbs(rest[2]) && (err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "") {
					return op, fmt.Errorf("repository must be an HTTP(S) URL or an absolute local path")
				}
			}
		case "rm", "remove":
			if len(rest) != 2 || !PackageName(rest[1]) || strings.Contains(rest[1], "/") {
				return op, fmt.Errorf("bucket rm requires one bucket name")
			}
			op.Arguments[1] = "rm"
		default:
			return op, fmt.Errorf("unsupported bucket command")
		}
	default:
		return op, fmt.Errorf("command %q is unsupported by %s", op.Name, PolicyVersion)
	}
	return op, nil
}

// Manifest is an unexecuted declaration. Executable fields and unknown keys are
// rejected; metadata used only by bucket maintainers is never evaluated.
type Manifest map[string]json.RawMessage

func DecodeManifest(data []byte) (Manifest, error) {
	var raw map[string]json.RawMessage
	if err := metadata.Decode(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("manifest must be an object")
	}
	result := Manifest{}
	for key, value := range raw {
		key = strings.ToLower(key)
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate case-insensitive manifest field %q", key)
		}
		result[key] = value
	}
	return result, nil
}

func manifestString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("expected a string")
	}
	return value, nil
}

func stringList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("expected a string or string array")
	}
	return values, nil
}

var allowedFields = map[string]bool{
	"version": true, "description": true, "homepage": true, "license": true, "architecture": true,
	"url": true, "hash": true, "extract_dir": true, "extract_to": true, "bin": true, "shortcuts": true,
	"env_add_path": true, "env_set": true, "psmodule": true, "persist": true, "depends": true,
	"suggest": true, "notes": true, "checkver": true, "autoupdate": true,
}

func validateFields(m Manifest) error {
	for key := range m {
		if !allowedFields[key] {
			return fmt.Errorf("manifest field %q is unsupported by %s", key, PolicyVersion)
		}
	}
	return nil
}

// effective64 merges the selected architecture after validating every branch,
// so changing architecture cannot expose a hidden executable field.
func effective64(m Manifest) (Manifest, error) {
	if err := validateFields(m); err != nil {
		return nil, err
	}
	selected := Manifest{}
	for key, value := range m {
		selected[key] = value
	}
	if raw, ok := m["architecture"]; ok {
		var branches map[string]json.RawMessage
		if err := json.Unmarshal(raw, &branches); err != nil {
			return nil, fmt.Errorf("invalid architecture map")
		}
		for name, value := range branches {
			if name != "64bit" && name != "32bit" && name != "arm64" {
				return nil, fmt.Errorf("unknown architecture %q", name)
			}
			branch, err := DecodeManifest(value)
			if err != nil {
				return nil, err
			}
			if err := validateFields(branch); err != nil {
				return nil, err
			}
			if _, nested := branch["architecture"]; nested {
				return nil, fmt.Errorf("nested architecture is unsupported")
			}
			if name == "64bit" {
				for key, value := range branch {
					selected[key] = value
				}
			}
		}
		if _, ok := branches["64bit"]; !ok {
			return nil, fmt.Errorf("manifest has no supported 64bit branch")
		}
	}
	delete(selected, "architecture")
	return selected, nil
}

func safeDeclarativeValue(value string) bool {
	if strings.ContainsAny(value, "\x00\r\n`") {
		return false
	}
	for _, placeholder := range []string{"$original_dir", "$persist_dir", "$dir"} {
		value = strings.ReplaceAll(value, placeholder, "")
	}
	return !strings.Contains(value, "$")
}

func validateRelativeList(raw json.RawMessage, field string) error {
	values, err := stringList(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	for _, value := range values {
		if value == "." && (field == "env_add_path" || field == "extract_dir") {
			continue
		}
		if !metadata.RelativePath(value) {
			return fmt.Errorf("%s contains a path outside the package: %q", field, value)
		}
	}
	return nil
}

func validateBindings(raw json.RawMessage, field string) error {
	if len(raw) == 0 {
		return nil
	}
	var entries []json.RawMessage
	if raw[0] == '"' {
		entries = []json.RawMessage{raw}
	} else if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("invalid %s list", field)
	}
	for _, entry := range entries {
		var single string
		if err := json.Unmarshal(entry, &single); err == nil {
			if !metadata.RelativePath(single) {
				return fmt.Errorf("invalid %s path", field)
			}
			continue
		}
		var tuple []string
		if err := json.Unmarshal(entry, &tuple); err != nil || len(tuple) < 2 || len(tuple) > 3 {
			return fmt.Errorf("invalid %s tuple", field)
		}
		if !metadata.RelativePath(tuple[0]) || !metadata.RelativePath(tuple[1]) {
			return fmt.Errorf("%s tuple escapes package", field)
		}
		if field == "bin" && (strings.ContainsAny(tuple[1], `/\`) || !namePattern.MatchString(tuple[1])) {
			return fmt.Errorf("invalid shim name")
		}
		if len(tuple) == 3 && tuple[2] != "" {
			return fmt.Errorf("%s tuple arguments are unsupported", field)
		}
	}
	return nil
}

// ValidateManifest defines the portable ZIP subset; filesystem integration is
// responsible for verifying extraction and generated artifact containment too.
func ValidateManifest(manifest Manifest) (Manifest, error) {
	m, err := effective64(manifest)
	if err != nil {
		return nil, err
	}
	version, err := manifestString(m["version"])
	if err != nil || !versionPattern.MatchString(version) || !metadata.RelativePath(version) || version == "nightly" || version == "current" {
		return nil, fmt.Errorf("manifest requires a fixed safe version")
	}
	urls, err := stringList(m["url"])
	if err != nil || len(urls) == 0 {
		return nil, fmt.Errorf("manifest requires download URLs")
	}
	hashes, err := stringList(m["hash"])
	if err != nil || len(hashes) != len(urls) {
		return nil, fmt.Errorf("every download requires a SHA-256 hash")
	}
	for i, value := range urls {
		u, parseErr := url.Parse(value)
		if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || !strings.HasSuffix(strings.ToLower(u.Path), ".zip") || u.Fragment != "" || !hashPattern.MatchString(hashes[i]) {
			return nil, fmt.Errorf("only hashed HTTP(S) ZIP downloads are supported")
		}
	}
	for _, field := range []string{"extract_dir", "extract_to", "env_add_path"} {
		if err := validateRelativeList(m[field], field); err != nil {
			return nil, err
		}
	}
	for _, field := range []string{"bin", "persist"} {
		if err := validateBindings(m[field], field); err != nil {
			return nil, err
		}
	}
	if raw := m["shortcuts"]; len(raw) > 0 {
		var entries [][]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("invalid shortcut metadata")
		}
		for _, entry := range entries {
			if len(entry) < 2 || len(entry) > 4 || !metadata.RelativePath(entry[0]) || !metadata.RelativePath(entry[1]) {
				return nil, fmt.Errorf("invalid shortcut path")
			}
		}
	}
	if raw := m["psmodule"]; len(raw) > 0 {
		var module map[string]string
		if err := json.Unmarshal(raw, &module); err != nil || len(module) != 1 || !namePattern.MatchString(module["name"]) || !metadata.RelativePath(module["name"]) {
			return nil, fmt.Errorf("invalid module metadata")
		}
	}
	if raw := m["env_set"]; len(raw) > 0 {
		var settings map[string]string
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, fmt.Errorf("invalid environment declarations")
		}
		seen := map[string]bool{}
		for name, value := range settings {
			upper := strings.ToUpper(name)
			reserved := map[string]bool{"PATH": true, "PSMODULEPATH": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true, "HOME": true, "USERPROFILE": true, "APPDATA": true, "LOCALAPPDATA": true, "PROGRAMDATA": true, "TEMP": true, "TMP": true, "XDG_CONFIG_HOME": true}
			if !variablePattern.MatchString(name) || seen[upper] || strings.HasPrefix(upper, "SCOOP") || strings.HasPrefix(upper, "PORTSCOOP") || strings.HasPrefix(upper, "GIT_") || reserved[upper] || !safeDeclarativeValue(value) {
				return nil, fmt.Errorf("unsupported environment declaration %q", name)
			}
			seen[upper] = true
		}
	}
	deps, err := stringList(m["depends"])
	if err != nil {
		return nil, err
	}
	for _, dep := range deps {
		if !PackageName(dep) {
			return nil, fmt.Errorf("invalid dependency %q", dep)
		}
	}
	return m, nil
}

type ManifestLoader func(string) ([]byte, error)
type PlannedPackage struct {
	Name     string
	Manifest Manifest
}

// Preflight resolves and validates the complete dependency closure before the
// caller starts any package mutation. Ordering is dependency-first.
func Preflight(names []string, load ManifestLoader) ([]PlannedPackage, error) {
	states := map[string]int{}
	type owner struct {
		reference   string
		declaration string
	}
	owners := map[string]owner{}
	var plan []PlannedPackage
	var visit func(string) error
	visit = func(name string) error {
		if !PackageName(name) {
			return fmt.Errorf("invalid package reference %q", name)
		}
		key := strings.ToLower(name)
		if states[key] == 1 {
			return fmt.Errorf("dependency cycle at %q", name)
		}
		if states[key] == 2 {
			return nil
		}
		states[key] = 1
		data, err := load(name)
		if err != nil {
			return fmt.Errorf("manifest %s: %w", name, err)
		}
		raw, err := DecodeManifest(data)
		if err != nil {
			return fmt.Errorf("manifest %s: %w", name, err)
		}
		manifest, err := ValidateManifest(raw)
		if err != nil {
			return fmt.Errorf("manifest %s: %w", name, err)
		}
		parts := strings.Split(key, "/")
		app := parts[len(parts)-1]
		encoded, _ := json.Marshal(manifest)
		if previous, exists := owners[app]; exists {
			if previous.declaration != string(encoded) {
				return fmt.Errorf("conflicting bucket declarations for installed app %q", app)
			}
			if states[previous.reference] == 1 {
				return fmt.Errorf("dependency cycle at %q", name)
			}
			states[key] = 2
			return nil
		}
		owners[app] = owner{reference: key, declaration: string(encoded)}
		deps, _ := stringList(manifest["depends"])
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		states[key] = 2
		plan = append(plan, PlannedPackage{Name: name, Manifest: manifest})
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return plan, nil
}
