package compatibility

import (
	"encoding/json"
)

func psmodule(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("portable module discovery is not implemented")
}
