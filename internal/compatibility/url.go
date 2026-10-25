package compatibility

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
)

func downloadName(value string) (string, string, string, error) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || strings.ContainsAny(value, "\x00\r\n") {
		return "", "", "", fmt.Errorf("HTTP(S) download URL required")
	}
	name := ""
	if u.Fragment != "" {
		if !strings.HasPrefix(u.Fragment, "/") {
			return "", "", "", fmt.Errorf("filename fragment must be /basename")
		}
		name = strings.TrimPrefix(u.Fragment, "/")
	} else {
		name, err = url.PathUnescape(path.Base(u.EscapedPath()))
		if err != nil {
			return "", "", "", err
		}
	}
	if !relative(name) || strings.ContainsAny(name, `/\`) {
		return "", "", "", fmt.Errorf("unsafe download basename")
	}
	format := ""
	switch strings.ToLower(path.Ext(name)) {
	case ".zip":
		format = "zip"
	case ".exe":
		format = "file"
	default:
		return "", "", "", fmt.Errorf("unsupported payload format for %s", name)
	}
	u.Fragment = ""
	u.RawFragment = ""
	return u.String(), name, format, nil
}

func downloadURL(m Manifest, raw json.RawMessage) (result, error) {
	values, err := stringsValue(raw)
	if err != nil || len(values) == 0 {
		return unsupported("download URLs are required")
	}
	seen := map[string]bool{}
	for _, value := range values {
		_, name, _, err := downloadName(value)
		if err != nil {
			return unsupported(err.Error())
		}
		key := strings.ToLower(name)
		if seen[key] {
			return unsupported("conflicting download basenames")
		}
		seen[key] = true
	}
	return keep(raw)
}
