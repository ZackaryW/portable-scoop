package compatibility

import (
	"encoding/json"
	"strings"
)

func shortcuts(m Manifest, raw json.RawMessage) (result, error) {
	entries, err := tuples(raw)
	if err != nil {
		return result{}, err
	}
	var aliases [][]string
	for _, entry := range entries {
		if len(entry) < 2 || len(entry) > 3 {
			return unsupported("shortcut requires target, name, and optional native arguments; icon overrides are unsupported")
		}
		name := strings.ReplaceAll(entry[1], " ", "-")
		if !alias(name) {
			return unsupported("shortcut name cannot form a portable alias")
		}
		args := ""
		if len(entry) == 3 {
			args = entry[2]
		}
		aliases = append(aliases, []string{entry[0], name, args})
	}
	encoded, _ := json.Marshal(aliases)
	r, err := bin(m, encoded)
	if err != nil {
		return r, err
	}
	r.reason = "Store-owned launch aliases; host Start Menu integration omitted"
	return r, nil
}
