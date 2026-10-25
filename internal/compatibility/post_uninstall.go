package compatibility

import (
	"encoding/json"
)

func postUninstall(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("unrecognized post-uninstall hook")
}
