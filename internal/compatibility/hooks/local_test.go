package hooks

import (
	"fmt"
	"strings"
	"testing"
)

func TestLocalOperationsAcrossPackagesAndPhases(t *testing.T) {
	for _, name := range []string{"Aurora", "Nimbus"} {
		for _, phase := range []string{"pre_install", "post_install"} {
			c := Context{Persist: [][2]string{{"profile", "saved"}}}
			src := fmt.Sprintf(`New-Item "$dir\cache" -ItemType Directory -Force | Out-Null
if (!(Test-Path "$persist_dir\saved\%s.ini")) { Copy-Item "$dir\defaults\%s.ini" "$dir\profile\%s.ini" -Force }
if (!(Test-Path "$persist_dir\saved\literal.ini")) { Set-Content "$dir\profile\literal.ini" 'UpdateCheck=0' -Encoding Ascii }
New-Item "$dir\literal.txt" -ItemType File -Value 'a literal $value' | Out-Null`, name, name, name)
			p, err := Compile(c, phase, src)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Actions) != 4 || p.Actions[1].PersistPath != "saved/"+name+".ini" || p.Actions[2].Content != "UpdateCheck=0\r\n" || p.Actions[3].Content != "a literal $value" {
				t.Fatal(p)
			}
			for _, a := range p.Actions {
				if a.Phase != phase {
					t.Fatal("phase lost")
				}
			}
		}
	}
}
func TestLocalOperationRejections(t *testing.T) {
	c := Context{Persist: [][2]string{{"profile", "profile"}}}
	for _, src := range []string{
		`Copy-Item "$env:APPDATA\file" "$dir\file"`,
		`Copy-Item "$dir\*.xml" "$dir\profile" -Recurse`,
		`Set-Content "$dir\x" "$(Get-Content secret)"`,
		`Set-Content "$dir\x" $value`,
		`New-Item "$dir\x" -ItemType File -Value (Get-Date)`,
		`if (!(Test-Path "$persist_dir\other")) { Copy-Item "$dir\defaults" "$dir\profile\x" }`,
		`New-Item "$dir\..\host" -ItemType Directory`,
		`New-Item "$dir\x" -ItemType SymbolicLink -Value 'host'`,
		`Set-Content "$dir\x" 'nonASCII: ä' -Encoding Ascii`,
		`Set-Content "$dir\x" 'a' -Encoding Unknown`,
	} {
		if p, err := Compile(c, "pre_install", src); err == nil || p.Actions != nil {
			t.Fatal("accepted", src)
		}
	}
	src := importScript("profile", "Host") + "\n" + `Copy-Item "$dir\profile\data" "$dir\required.txt"`
	if _, err := Compile(c, "pre_install", src); err == nil || !strings.Contains(err.Error(), "depends on skipped") {
		t.Fatal("import dependency missed", err)
	}
}
