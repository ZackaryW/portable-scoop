package compatibility

import (
	"encoding/json"
)

func license(_ Manifest, raw json.RawMessage) (result, error) { return keep(raw) }
