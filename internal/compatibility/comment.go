package compatibility

import (
	"encoding/json"
)

func comment(_ Manifest, _ json.RawMessage) (result, error) {
	return ignore("metadata is not evaluated during installation")
}
