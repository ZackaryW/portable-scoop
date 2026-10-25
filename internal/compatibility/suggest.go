package compatibility

import (
	"encoding/json"
)

func suggest(_ Manifest, raw json.RawMessage) (result, error) { return keep(raw) }
