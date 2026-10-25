package compatibility

import (
	"encoding/json"
)

func comments(_ Manifest, _ json.RawMessage) (result, error) {
	return ignore("metadata is not evaluated during installation")
}
