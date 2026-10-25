package hooks

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func importScript(profile, host string) string {
	return fmt.Sprintf(`if (!(Test-Path "$dir\%s\*") -and (Test-Path "$env:Appdata\%s")) {
    if (!(Test-Path "$dir\%s")) { New-Item "$dir\%s" -ItemType Directory | Out-Null }
    Copy-Item "$env:Appdata\%s\*" "$dir\%s\" -Recurse -Force
}`, profile, host, profile, profile, host, profile)
}
func TestGenericMixedOperations(t *testing.T) {
	for _, name := range []string{"Aurora", "OtherVendor", "Bitwarden"} {
		c := Context{Version: "1.2", Persist: [][2]string{{"profile", "profile"}, {"config.xml", "config.xml"}}}
		source := fmt.Sprintf("Rename-Item \"$dir\\%s-$version.exe\" 'app.exe'\n", name) + importScript("profile", name) + `;
if (-not (Test-Path -Path "$persist_dir\config.xml" -PathType Leaf)) {
    New-Item -Path "$dir\config.xml" -ItemType File -Force | Out-Null
}
Rename-Item -LiteralPath "$dir\app.exe" -NewName 'launch.exe'`
		for _, phase := range []string{"pre_install", "post_install"} {
			p, err := Compile(c, phase, source)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Actions) != 3 || p.Actions[0].Source != name+"-1.2.exe" || p.Actions[2].Source != "app.exe" || p.Actions[2].Path != "launch.exe" || p.Decisions[1].Disposition != "skip" {
				t.Fatalf("%+v", p)
			}
			again, err := Compile(c, phase, source)
			if err != nil || !reflect.DeepEqual(p, again) {
				t.Fatal("unstable compilation")
			}
		}
	}
}
func TestGenericSkipOnlyAndRejections(t *testing.T) {
	c := Context{Version: "1", Persist: [][2]string{{"profile", "profile"}}}
	script := importScript("profile", "Unrelated")
	p, err := Compile(c, "pre_install", script)
	if err != nil || len(p.Actions) != 0 || len(p.Decisions) != 1 || p.Decisions[0].Disposition != "skip" {
		t.Fatalf("%+v %v", p, err)
	}
	for name, source := range map[string]string{
		"unconditional":           `Copy-Item "$env:APPDATA\Unrelated\*" "$dir\profile\" -Recurse`,
		"unknown remainder":       script + "\nWrite-Host ignored",
		"changed source":          strings.Replace(script, `Copy-Item "$env:Appdata\Unrelated\*"`, `Copy-Item "$env:Appdata\Different\*"`, 1),
		"changed guard":           strings.Replace(script, " -and ", " -or ", 1),
		"condition pipeline":      strings.Replace(script, `Test-Path "$env:Appdata\Unrelated"`, `Test-Path "$env:Appdata\Unrelated" | Out-Null`, 1),
		"undeclared profile":      importScript("other", "Unrelated"),
		"host deletion":           strings.Replace(script, "Copy-Item", "Remove-Item", 1),
		"extra effect":            strings.Replace(script, "    Copy-Item", "    Unknown-Command; Copy-Item", 1),
		"dynamic target":          `Rename-Item "$dir\$(Get-Content secret).exe" 'app.exe'`,
		"wrong version expansion": `Rename-Item "$dir\$other.exe" 'app.exe'`,
		"traversal":               `Rename-Item "$dir\..\secret.exe" 'app.exe'`,
		"single quoted variable":  `Rename-Item '$dir\app.exe' 'app.exe'`,
		"persist rename":          `Rename-Item "$dir\profile\config.json" 'new.json'`,
		"device":                  `Rename-Item "$dir\CON.exe" 'app.exe'`,
		"newname traversal":       `Rename-Item "$dir\app.exe" '..\other.exe'`,
		"extra flag":              `Rename-Item "$dir\app.exe" 'other.exe' -ErrorAction SilentlyContinue`,
	} {
		t.Run(name, func(t *testing.T) {
			p, err := Compile(c, "pre_install", source)
			if err == nil || p.Actions != nil {
				t.Fatalf("accepted %s", source)
			}
		})
	}
}
func TestSkippedImportDependencies(t *testing.T) {
	c := Context{Persist: [][2]string{{"profile", "profile"}}}
	reader := func(_ Context, n node) (operation, bool, error) {
		if command(n, "Read-Required") {
			return operation{action: &Action{Kind: "fixture"}, reads: []string{"PROFILE/config.json"}}, true, nil
		}
		return operation{}, false, nil
	}
	for _, script := range []string{importScript("profile", "Host") + "\nRead-Required", "Read-Required\n" + importScript("profile", "Host")} {
		p, err := compile(c, "post_install", script, append(append([]recognizer{}, operations...), reader))
		if err == nil || p.Actions != nil || !strings.Contains(err.Error(), "depends on skipped") {
			t.Fatalf("%+v %v", p, err)
		}
	}
}
