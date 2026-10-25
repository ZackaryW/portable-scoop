package compatibility

import (
	"encoding/json"
)

func postInstall(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("unrecognized post-install hook")
}
