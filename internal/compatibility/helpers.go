package compatibility

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"portscoop/internal/metadata"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var versionPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
var variablePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var aliasPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.+-]*$`)

func Decode(data []byte) (Manifest, error) {
	// Decode keys as a stream to reject both exact and case-folded duplicates.
	d := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("manifest must be an object")
	}
	m := Manifest{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(token.(string))
		if _, exists := m[key]; exists {
			return nil, fmt.Errorf("duplicate case-insensitive manifest field %q", key)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		m[key] = raw
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	var complete map[string]json.RawMessage
	if err := metadata.Decode(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &complete); err != nil {
		return nil, err
	}
	return m, nil
}

func stringValue(raw json.RawMessage) (string, error) {
	var s string
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &s) != nil {
		return "", fmt.Errorf("expected a string")
	}
	return s, nil
}
func stringsValue(raw json.RawMessage) ([]string, error) {
	if s, err := stringValue(raw); err == nil {
		return []string{s}, nil
	}
	var values []json.RawMessage
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &values) != nil {
		return nil, fmt.Errorf("expected a string or string array")
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		s, err := stringValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

func PackageName(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if !namePattern.MatchString(p) || !relative(p) {
			return false
		}
	}
	return !strings.EqualFold(parts[len(parts)-1], "scoop")
}
func relative(value string) bool {
	if !metadata.RelativePath(value) || strings.ContainsAny(value, "\r\n\t\"<>|?*`$") {
		return false
	}
	for _, part := range strings.Split(strings.ReplaceAll(value, `\`, "/"), "/") {
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
			return false
		}
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
	}
	return true
}
func alias(value string) bool {
	return aliasPattern.MatchString(value) && relative(value) && !strings.ContainsAny(value, `/\`)
}
func relativeList(raw json.RawMessage, dot bool) (result, error) {
	values, err := stringsValue(raw)
	if err != nil {
		return result{}, err
	}
	for _, v := range values {
		if !(dot && v == ".") && !relative(v) {
			return unsupported("path must stay inside the package")
		}
	}
	return keep(raw)
}
func tuples(raw json.RawMessage) ([][]string, error) {
	if s, err := stringValue(raw); err == nil {
		return [][]string{{s}}, nil
	}
	var entries []json.RawMessage
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &entries) != nil {
		return nil, fmt.Errorf("expected a binding list")
	}
	var out [][]string
	for _, entry := range entries {
		values, err := stringsValue(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, values)
	}
	return out, nil
}
func nativeTarget(target string) bool {
	ext := strings.ToLower(path.Ext(target))
	return ext == ".exe" || ext == ".com"
}

func mergeBindings(entries [][]string) ([][]string, error) {
	seen := map[string][]string{}
	var out [][]string
	for _, entry := range entries {
		key := strings.ToLower(entry[1])
		if old, ok := seen[key]; ok {
			if !strings.EqualFold(strings.ReplaceAll(old[0], `\`, "/"), strings.ReplaceAll(entry[0], `\`, "/")) || old[2] != entry[2] {
				return nil, fmt.Errorf("conflicting launch alias %q", entry[1])
			}
			continue
		}
		seen[key] = entry
		out = append(out, entry)
	}
	return out, nil
}
