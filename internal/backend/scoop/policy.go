package scoop

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"portscoop/internal/compatibility"
	"portscoop/internal/metadata"
)

const PolicyVersion = compatibility.Version

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

// Manifest is an unexecuted declaration checked by the per-field compiler.
type Manifest = compatibility.Manifest

func DecodeManifest(data []byte) (Manifest, error) { return compatibility.Decode(data) }

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

// ValidateManifest returns the compiled effective declaration. Plan also carries
// the decisions and typed actions needed by the managed adapter.
func ValidateManifest(manifest Manifest) (Manifest, error) {
	plan, err := compatibility.Compile(manifest)
	return plan.Manifest, err
}

type ManifestLoader func(string) ([]byte, error)
type PlannedPackage struct {
	Name      string
	Manifest  Manifest
	Decisions []compatibility.Decision
	Actions   []compatibility.Action
	Downloads []compatibility.Download
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
		compiled, err := compatibility.Compile(raw)
		if err != nil {
			return fmt.Errorf("manifest %s: %w", name, err)
		}
		manifest := compiled.Manifest
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
		plan = append(plan, PlannedPackage{Name: name, Manifest: manifest, Decisions: compiled.Decisions, Actions: compiled.Actions, Downloads: compiled.Downloads})
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return plan, nil
}
