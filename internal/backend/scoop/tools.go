package scoop

import (
	"fmt"
	"path/filepath"
	"strings"

	"portscoop/internal/environment"
	"portscoop/internal/metadata"
	"portscoop/internal/toolbundle"
)

const ToolsRelative = ".portscoop/tools"

var helperExecutables = map[string]string{"git": "git/cmd/git.exe", "7zip": "7zip/7z.exe"}

func verifyToolInventory(root string, backend environment.BackendMetadata) (toolbundle.Inventory, error) {
	var inventory toolbundle.Inventory
	data, err := containedFile(root, filepath.Join(root, ToolsRelative, "tools.json"))
	if err != nil {
		return inventory, err
	}
	if err = metadata.Decode(data, &inventory); err != nil {
		return inventory, err
	}
	if err = inventory.Validate(); err != nil {
		return inventory, err
	}
	for _, required := range []string{"git/cmd/git.exe", "7zip/7z.exe", "7zip/7z.dll"} {
		if inventory.Artifacts[required] == "" {
			return inventory, fmt.Errorf("missing managed helper %s", required)
		}
	}
	for path, digest := range inventory.Artifacts {
		if backend.Artifacts[ToolsRelative+"/"+path] != digest {
			return inventory, fmt.Errorf("tool inventory differs from backend: %s", path)
		}
	}
	for path := range backend.Artifacts {
		if strings.HasPrefix(path, ToolsRelative+"/") && path != ToolsRelative+"/tools.json" && inventory.Artifacts[strings.TrimPrefix(path, ToolsRelative+"/")] == "" {
			return inventory, fmt.Errorf("tool missing from helper inventory: %s", path)
		}
	}
	return inventory, nil
}

// Helper returns only a verified, fixed entry point in the selected Store.
func Helper(paths environment.Paths, name string) (string, error) {
	rel, ok := helperExecutables[strings.ToLower(name)]
	if !ok {
		return "", fmt.Errorf("unsupported managed helper %s", name)
	}
	if _, err := Verify(paths); err != nil {
		return "", err
	}
	return filepath.Join(paths.Root, ToolsRelative, filepath.FromSlash(rel)), nil
}
