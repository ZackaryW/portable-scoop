package compatibility

import (
	"encoding/json"
	"path"
	"strings"

	"portscoop/internal/shimargs"
)

func bin(_ Manifest, raw json.RawMessage) (result, error) {
	entries, err := tuples(raw)
	if err != nil {
		return result{}, err
	}
	var bindings [][]string
	for _, entry := range entries {
		if len(entry) < 1 || len(entry) > 3 || !relative(entry[0]) {
			return unsupported("invalid bin path or tuple")
		}
		name := strings.TrimSuffix(path.Base(strings.ReplaceAll(entry[0], `\`, "/")), path.Ext(entry[0]))
		args := ""
		if len(entry) > 1 {
			name = entry[1]
		}
		if len(entry) > 2 {
			args = entry[2]
		}
		if !alias(name) {
			return unsupported("invalid shim name")
		}
		if args != "" {
			if !nativeTarget(entry[0]) {
				return unsupported("fixed arguments require a native EXE/COM target")
			}
			if _, err := shimargs.Parse(args); err != nil {
				return result{}, err
			}
		}
		bindings = append(bindings, []string{entry[0], name, args})
	}
	return result{disposition: Rewrite, reason: "Store-owned shims", bindings: bindings}, nil
}
