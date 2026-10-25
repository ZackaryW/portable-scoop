package toolbundle

import (
	"encoding/hex"
	"fmt"
	"strings"

	"portscoop/internal/archive"
)

// Validate checks the inventory's closed tool namespaces and required files.
// The owning release/backend separately verifies the recorded file bytes.
func (i Inventory) Validate() error {
	if err := i.Pins.Validate(); err != nil {
		return err
	}
	names := map[string]bool{}
	for _, tool := range i.Tools {
		names[tool.Name] = true
		for _, p := range tool.Required {
			if i.Artifacts[tool.Name+"/"+p] == "" {
				return fmt.Errorf("missing required tool artifact %s/%s", tool.Name, p)
			}
		}
	}
	seen := map[string]bool{}
	for path, digest := range i.Artifacts {
		name, _, found := strings.Cut(path, "/")
		h, err := hex.DecodeString(digest)
		if !found || !names[name] || !archive.SafePath(path) || strings.Contains(path, "\\") || seen[strings.ToLower(path)] || err != nil || len(h) != 32 || strings.ToLower(digest) != digest {
			return fmt.Errorf("invalid tool artifact %s", path)
		}
		seen[strings.ToLower(path)] = true
	}
	return nil
}
