package compatibility

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var executableName = regexp.MustCompile(`\A[A-Za-z0-9][A-Za-z0-9._+-]*\.[eE][xX][eE]\z`)

// A literal path avoids Scoop's query/fragment filename rewriting rules.
func directExecutable(u *url.URL) bool {
	return u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" &&
		u.EscapedPath() == u.Path && !strings.ContainsAny(u.Path, `\`) &&
		executableName.MatchString(path.Base(u.Path)) && relative(path.Base(u.Path))
}

func downloadURL(m Manifest, raw json.RawMessage) (result, error) {
	values, err := stringsValue(raw)
	if err != nil || len(values) == 0 {
		return unsupported("download URLs are required")
	}
	for _, value := range values {
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Fragment != "" || strings.ContainsAny(value, "\x00\r\n") {
			return unsupported("only HTTP(S) ZIP or direct EXE URLs without filename fragments are supported")
		}
		if strings.HasSuffix(strings.ToLower(u.Path), ".exe") {
			if !directExecutable(u) || m["extract_dir"] != nil || m["extract_to"] != nil {
				return unsupported("direct EXE payloads require a literal safe filename, no query or encoded path, and no extraction directives")
			}
		} else if !strings.HasSuffix(strings.ToLower(u.Path), ".zip") {
			return unsupported("only HTTP(S) ZIP or direct EXE payloads are supported")
		}
	}
	return keep(raw)
}
