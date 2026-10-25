package compatibility

import (
	"encoding/json"
)

func extractTo(_ Manifest, raw json.RawMessage) (result, error) { return extractionField(raw) }
