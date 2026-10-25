package compatibility

import (
	"encoding/json"
	"fmt"
	"sort"
)

func architecture(input Manifest) (Manifest, error) {
	selected := Manifest{}
	for k, v := range input {
		if k != "architecture" {
			selected[k] = v
		}
	}
	if raw, ok := input["architecture"]; ok {
		branches, err := Decode(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid architecture map: %w", err)
		}
		keys := make([]string, 0, len(branches))
		for k := range branches {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, name := range keys {
			if name != "64bit" && name != "32bit" && name != "arm64" {
				return nil, fmt.Errorf("unknown architecture %q", name)
			}
			branch, err := Decode(json.RawMessage(branches[name]))
			if err != nil {
				return nil, err
			}
			if _, nested := branch["architecture"]; nested {
				return nil, fmt.Errorf("nested architecture is unsupported")
			}
			if name == "64bit" {
				for k, v := range branch {
					selected[k] = v
				}
			}
		}
		if _, ok := branches["64bit"]; !ok {
			return nil, fmt.Errorf("manifest has no supported 64bit branch")
		}
	}
	return selected, nil
}
