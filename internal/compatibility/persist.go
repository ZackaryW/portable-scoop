package compatibility

import (
	"encoding/json"
)

func persist(_ Manifest, raw json.RawMessage) (result, error) {
	entries, err := tuples(raw)
	if err != nil {
		return result{}, err
	}
	for _, entry := range entries {
		if len(entry) < 1 || len(entry) > 2 {
			return unsupported("persist requires a path or source/destination pair")
		}
		for _, value := range entry {
			if !relative(value) {
				return unsupported("persist path escapes package")
			}
		}
	}
	return keep(raw)
}
