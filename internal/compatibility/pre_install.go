package compatibility

import (
	"encoding/json"
	"regexp"
	"strings"
)

// This is an exact language of one reviewed operation, not a PowerShell safety
// classifier. Anything outside this complete pattern must be rejected.
var emptyConfig = regexp.MustCompile(`\A\s*if \(-not \(Test-Path -Path "\$persist_dir\\([A-Za-z0-9][A-Za-z0-9._-]*)" -PathType Leaf\)\) \{\s*New-Item -Path "\$dir\\([A-Za-z0-9][A-Za-z0-9._-]*)" -ItemType File -Force \| Out-Null\s*\}\s*\z`)

func preInstall(m Manifest, raw json.RawMessage) (result, error) {
	lines, err := stringsValue(raw)
	if err != nil {
		return result{}, err
	}
	match := emptyConfig.FindStringSubmatch(strings.Join(lines, "\n"))
	if len(match) != 3 || match[1] != match[2] || !relative(match[1]) {
		return unsupported("unrecognized pre-install hook; no complete portable replacement")
	}
	entries, err := tuples(m["persist"])
	if err != nil {
		return unsupported("configuration initialization requires matching persistence")
	}
	found := false
	for _, entry := range entries {
		if (len(entry) == 1 && entry[0] == match[1]) || (len(entry) == 2 && entry[0] == match[1] && entry[1] == match[1]) {
			found = true
		}
	}
	if !found {
		return unsupported("configuration initialization requires matching persistence")
	}
	return result{disposition: Rewrite, reason: "initialize empty configuration without evaluating PowerShell", value: raw, actions: []Action{{Phase: "pre_install", Kind: "ensure-persisted-file", Path: match[1]}}}, nil
}
