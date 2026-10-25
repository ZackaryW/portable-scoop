package compatibility

import (
	"encoding/json"
)

func description(_ Manifest, raw json.RawMessage) (result, error) { return keep(raw) }
