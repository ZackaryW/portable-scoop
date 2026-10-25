package compatibility

import (
	"encoding/json"
)

func uninstaller(_ Manifest, _ json.RawMessage) (result, error) {
	return unsupported("no portable replacement for this uninstaller")
}
