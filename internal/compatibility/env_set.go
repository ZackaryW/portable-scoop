package compatibility

import (
	"encoding/json"
	"sort"
	"strings"
)

func envSet(_ Manifest, raw json.RawMessage) (result, error) {
	var settings map[string]string
	if json.Unmarshal(raw, &settings) != nil || settings == nil {
		return unsupported("invalid environment declarations")
	}
	seen := map[string]bool{}
	names := make([]string, 0, len(settings))
	for name := range settings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := settings[name]
		upper := strings.ToUpper(name)
		reserved := map[string]bool{"PATH": true, "PSMODULEPATH": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true, "HOME": true, "USERPROFILE": true, "APPDATA": true, "LOCALAPPDATA": true, "PROGRAMDATA": true, "TEMP": true, "TMP": true, "XDG_CONFIG_HOME": true}
		if !variablePattern.MatchString(name) || seen[upper] || strings.HasPrefix(upper, "SCOOP") || strings.HasPrefix(upper, "PORTSCOOP") || strings.HasPrefix(upper, "GIT_") || reserved[upper] || !safeValue(value) {
			return unsupported("unsupported environment declaration " + name)
		}
		seen[upper] = true
	}
	return keep(raw)
}

func safeValue(value string) bool {
	if strings.ContainsAny(value, "\x00\r\n`") {
		return false
	}
	for _, placeholder := range []string{"$original_dir", "$persist_dir", "$dir"} {
		value = strings.ReplaceAll(value, placeholder, "")
	}
	return !strings.Contains(value, "$")
}
