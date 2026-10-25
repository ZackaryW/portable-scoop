package compatibility

import (
	"encoding/json"
)

func installer(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("no portable replacement for this installer")
}
