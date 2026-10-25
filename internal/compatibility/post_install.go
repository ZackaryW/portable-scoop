package compatibility

import (
	"encoding/json"
)

func postInstall(m Manifest, raw json.RawMessage) (result, error) {
	return compileHook(m, raw, "post_install")
}
