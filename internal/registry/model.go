package registry

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"portscoop/internal/environment"
	"portscoop/internal/metadata"
)

const SchemaVersion = 1

var aliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type Entry struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	VolumeID     string `json:"volume_id,omitempty"`
	RelativePath string `json:"relative_path"`
}

type Document struct {
	SchemaVersion int              `json:"schema_version"`
	Entries       map[string]Entry `json:"entries"`
	DefaultID     string           `json:"default_id,omitempty"`
}

func Empty() Document { return Document{SchemaVersion: SchemaVersion, Entries: map[string]Entry{}} }
func Alias(value string) (string, error) {
	value = strings.ToLower(value)
	if !aliasPattern.MatchString(value) {
		return "", fmt.Errorf("alias must contain 1-64 ASCII letters, digits, underscores or hyphens and start with a letter or digit")
	}
	return value, nil
}

func (d Document) Validate() error {
	if d.SchemaVersion != SchemaVersion || d.Entries == nil {
		return fmt.Errorf("unsupported or invalid registry schema %d", d.SchemaVersion)
	}
	defaultFound := d.DefaultID == ""
	for alias, e := range d.Entries {
		normalized, err := Alias(alias)
		if err != nil || normalized != alias {
			return fmt.Errorf("invalid or noncanonical registry alias %q", alias)
		}
		if !environment.ValidID(e.ID) || !filepath.IsAbs(e.Path) || !metadata.RelativePath(e.RelativePath) || strings.ContainsRune(e.Path, 0) {
			return fmt.Errorf("invalid location hints for %q", alias)
		}
		if e.ID == d.DefaultID {
			defaultFound = true
		}
	}
	if !defaultFound {
		return fmt.Errorf("registry default refers to an unregistered identity")
	}
	return nil
}

func Decode(data []byte) (Document, error) {
	var d Document
	if err := metadata.Decode(data, &d); err != nil {
		return d, fmt.Errorf("registry metadata: %w", err)
	}
	return d, d.Validate()
}
