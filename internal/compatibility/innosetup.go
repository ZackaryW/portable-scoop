package compatibility

import (
	"encoding/json"
)

func innosetup(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("Inno extraction is not implemented")
}
