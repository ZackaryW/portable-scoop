package compatibility

import (
	"encoding/json"
)

func cookie(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("download cookies are not implemented")
}
