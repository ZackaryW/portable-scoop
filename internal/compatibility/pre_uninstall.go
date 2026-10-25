package compatibility

import (
	"encoding/json"
)

func preUninstall(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("unrecognized pre-uninstall hook")
}
