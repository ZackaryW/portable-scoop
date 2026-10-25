package relocation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"portscoop/internal/atomicfile"
	"portscoop/internal/environment"
	"portscoop/internal/metadata"
)

type Artifact struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Content []byte `json:"content,omitempty"`
	Digest  string `json:"digest,omitempty"`
	Target  string `json:"target,omitempty"`
}
type Prepared struct {
	SchemaVersion int        `json:"schema_version"`
	Root          string     `json:"root"`
	Artifacts     []Artifact `json:"artifacts"`
}
type Journal struct {
	SchemaVersion int        `json:"schema_version"`
	From          string     `json:"from"`
	To            string     `json:"to"`
	Artifacts     []Artifact `json:"artifacts"`
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return replaceFile(path, data)
}
func replaceFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), "portscoop-state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return atomicfile.Replace(file.Name(), path)
}
func readJSON(root, path string, value any) error {
	if err := containedParent(root, path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("managed state is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return metadata.Decode(data, value)
}
func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && (relative == "." || metadata.RelativePath(relative))
}

func artifactPath(paths environment.Paths, a Artifact) (string, error) {
	if !metadata.RelativePath(a.Path) {
		return "", fmt.Errorf("invalid owned artifact %s", a.Path)
	}
	path := filepath.Join(paths.Root, filepath.FromSlash(a.Path))
	if err := containedParent(paths.Root, path); err != nil {
		return "", err
	}
	key := filepath.ToSlash(a.Path)
	switch a.Kind {
	case "shim", "binary":
		if !strings.HasPrefix(key, "scoop/shims/") || strings.Contains(strings.TrimPrefix(key, "scoop/shims/"), "/") {
			return "", fmt.Errorf("invalid shim ownership path %s", a.Path)
		}
	case "environment":
		if key != ".portscoop/state/environment.json" {
			return "", fmt.Errorf("invalid environment ownership path")
		}
	case "junction", "hardlink":
		if !metadata.RelativePath(a.Target) {
			return "", fmt.Errorf("invalid managed link destination")
		}
		parts := strings.Split(key, "/")
		if len(parts) < 4 || parts[0] != "scoop" || parts[1] != "apps" || parts[2] == "scoop" {
			return "", fmt.Errorf("invalid managed link ownership path %s", a.Path)
		}
		target := filepath.ToSlash(a.Target)
		if len(parts) == 4 && parts[3] == "current" {
			if a.Kind != "junction" || !strings.HasPrefix(target, "scoop/apps/"+parts[2]+"/") || len(strings.Split(target, "/")) != 4 || strings.HasSuffix(target, "/current") {
				return "", fmt.Errorf("invalid current link destination")
			}
		} else if !strings.HasPrefix(target, "scoop/persist/"+parts[2]+"/") {
			return "", fmt.Errorf("persist link destination escapes its app")
		}
	default:
		return "", fmt.Errorf("unsupported owned artifact format %s", a.Kind)
	}
	return path, nil
}

// Replace only a complete old-root prefix, never an arbitrary drive letter or
// a sibling such as C:\tools-old when C:\tools is the prepared root.
func relocateText(value, from, to string) string {
	if strings.EqualFold(from, to) {
		return value
	}
	var out strings.Builder
	cursor := 0
	for _, match := range regexp.MustCompile("(?i)"+regexp.QuoteMeta(from)).FindAllStringIndex(value, -1) {
		index, end := match[0], match[1]
		if end == len(value) || strings.ContainsRune("\\/\"'; \r\n", rune(value[end])) {
			out.WriteString(value[cursor:index])
			out.WriteString(to)
			cursor = end
		}
	}
	out.WriteString(value[cursor:])
	return out.String()
}
