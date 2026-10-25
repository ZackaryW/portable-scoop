package compatibility

import (
	"encoding/json"
)

func envAddPath(_ Manifest, raw json.RawMessage) (result, error) { return relativeList(raw, true) }
