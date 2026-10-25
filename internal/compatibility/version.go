package compatibility

import (
	"encoding/json"
	"strings"
)

func version(_ Manifest, raw json.RawMessage) (result, error) {
	v, err := stringValue(raw)
	if err != nil || !versionPattern.MatchString(v) || !relative(v) || strings.EqualFold(v, "nightly") {
		return unsupported("a fixed safe version is required")
	}
	return keep(raw)
}
