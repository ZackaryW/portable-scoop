package compatibility

import (
	"encoding/json"
)

func extractDir(_ Manifest, raw json.RawMessage) (result, error) { return extractionField(raw) }
