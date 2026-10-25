package compatibility

import (
	"encoding/json"
)

func homepage(_ Manifest, raw json.RawMessage) (result, error) { return keep(raw) }
