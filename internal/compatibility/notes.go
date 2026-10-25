package compatibility

import (
	"encoding/json"
)

func notes(_ Manifest, raw json.RawMessage) (result, error) { return keep(raw) }
