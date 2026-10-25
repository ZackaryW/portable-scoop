package compatibility

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"
)

// Match the whole reviewed recipe. In particular, this is not a filter that
// deletes arbitrary statements mentioning AppData. Only this optional import
// can be omitted while preserving the required local rename.
var bitwardenRecipe = []string{
	`Rename-Item "$dir\Bitwarden-Portable-$version.exe" 'Bitwarden.exe'`,
	`# copy config from non-portable version`,
	`if (!(Test-Path "$dir\bitwarden-appdata\*") -and (Test-Path "$env:Appdata\Bitwarden")) {`,
	`if (!(Test-Path "$dir\bitwarden-appdata")) { New-Item "$dir\bitwarden-appdata" -ItemType Directory | Out-Null }`,
	`Copy-Item "$env:Appdata\Bitwarden\*" "$dir\bitwarden-appdata\" -Recurse -Force`,
	`}`,
}

func matchesBitwardenRecipe(lines []string) bool {
	actual := strings.Split(strings.Join(lines, "\n"), "\n")
	if len(actual) != len(bitwardenRecipe) {
		return false
	}
	for i, line := range actual {
		if strings.TrimSpace(line) != bitwardenRecipe[i] {
			return false
		}
	}
	return true
}

func bitwardenPreInstall(m Manifest, raw json.RawMessage) (result, error) {
	v, err := stringValue(m["version"])
	if err != nil || !versionPattern.MatchString(v) || !relative(v) {
		return unsupported("Bitwarden recipe requires a fixed safe version")
	}
	source := "Bitwarden-Portable-" + v + ".exe"
	urls, err := stringsValue(m["url"])
	if err != nil || len(urls) != 1 {
		return unsupported("Bitwarden recipe requires its single portable executable payload")
	}
	u, err := url.Parse(urls[0])
	if err != nil || !directExecutable(u) || path.Base(u.Path) != source {
		return unsupported("Bitwarden recipe payload does not match its versioned source filename")
	}
	entries, err := tuples(m["persist"])
	if err != nil || len(entries) != 1 || len(entries[0]) == 0 || entries[0][0] != "bitwarden-appdata" ||
		(len(entries[0]) != 1 && (len(entries[0]) != 2 || entries[0][1] != "bitwarden-appdata")) {
		return unsupported("Bitwarden recipe requires matching bitwarden-appdata persistence")
	}
	return result{disposition: Rewrite,
		reason: "rename portable Bitwarden executable locally; skip reviewed optional import from host AppData\\Bitwarden",
		value:  raw, actions: []Action{{Phase: "pre_install", Kind: "rename-package-file", Source: source, Path: "Bitwarden.exe"}}}, nil
}
