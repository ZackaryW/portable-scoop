package compatibility

import (
	"encoding/json"
	"net/url"
	"strings"
)

func downloadURL(_ Manifest, raw json.RawMessage) (result, error) {
	values, err := stringsValue(raw)
	if err != nil || len(values) == 0 {
		return unsupported("download URLs are required")
	}
	for _, value := range values {
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || !strings.HasSuffix(strings.ToLower(u.Path), ".zip") || u.Fragment != "" || strings.ContainsAny(value, "\x00\r\n") {
			return unsupported("only HTTP(S) ZIP URLs without filename fragments are supported")
		}
	}
	return keep(raw)
}
