package compatibility

import (
	"encoding/json"
)

func depends(_ Manifest, raw json.RawMessage) (result, error) {
	values, err := stringsValue(raw)
	if err != nil {
		return result{}, err
	}
	for _, value := range values {
		if !PackageName(value) {
			return unsupported("invalid package dependency: " + value)
		}
	}
	return keep(raw)
}
