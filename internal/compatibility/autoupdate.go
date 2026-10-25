package compatibility

import (
	"encoding/json"
)

func autoupdate(_ Manifest, _ json.RawMessage) (result, error) {
	return ignore("metadata is not evaluated during installation")
}
