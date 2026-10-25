package hooks

import (
	"strings"
	"testing"
)

func TestCleanupAndTreeCopyAreIndependentUnits(t *testing.T) {
	c := Context{Persist: [][2]string{{"assets", "saved-assets"}}}
	for _, phase := range []string{"pre_install", "post_install"} {
		p, e := Compile(c, phase, `Copy-Item "$dir\assets.original\*" "$dir\assets" -Force -Recurse; Remove-Item "$dir\assets.original" -Recurse -Force`)
		if e != nil || len(p.Actions) != 2 || p.Actions[0].PersistPath != "saved-assets" || p.Actions[1].Paths[0] != "assets.original" || len(p.Writes) != 2 {
			t.Fatalf("%+v %v", p, e)
		}
	}
	p, e := Compile(Context{}, "pre_install", `Remove-Item "$dir\old*", "$dir\tmp" -Recurse -Force -ErrorAction SilentlyContinue`)
	if e != nil || len(p.Actions) != 1 || !p.Actions[0].MissingOK || len(p.Actions[0].Paths) != 2 {
		t.Fatalf("%+v %v", p, e)
	}
	p, e = Compile(Context{}, "post_install", `'$literal' | ForEach-Object { $path=Join-Path $dir $_; Remove-Item $path -Recurse }`)
	if e != nil || len(p.Actions) != 1 || p.Actions[0].Paths[0] != "$literal" {
		t.Fatalf("literal binding reinterpreted: %+v %v", p, e)
	}
}

func TestTreeOperationsRejectUnsafeRegions(t *testing.T) {
	c := Context{Persist: [][2]string{{"saved", "saved"}, {"nested/profile", "nested/profile"}}}
	for _, source := range []string{
		`Remove-Item "$dir\*" -Recurse`,
		`Remove-Item "$dir\sa*" -Recurse`,
		`Remove-Item "$dir\nested\pro*" -Recurse`,
		`Remove-Item "$dir\nested" -Recurse`,
		`Remove-Item "$dir\nested\profile\data" -Recurse`,
		`Remove-Item "$dir\a*\b" -Recurse`,
		`Remove-Item -LiteralPath "$dir\prefix*" -Recurse`,
		`Remove-Item "$env:APPDATA\profile" -Recurse -Force`,
		`Remove-Item "$dir\temporary" -ErrorAction Ignore`,
		`Copy-Item "$dir\saved\*" "$dir\elsewhere" -Force -Recurse`,
		`Copy-Item "$dir\a\*" "$dir\a\nested" -Force -Recurse`,
		`Copy-Item "$dir\a\*" "$dir\nested" -Force -Recurse`,
		`Copy-Item "$dir\a\*" "$dir\nested\profile\child" -Force -Recurse`,
		`Copy-Item "$dir\a\*" "$dir\b" -Recurse`,
		`if(Test-Path "$dir\other"){ Copy-Item "$dir\a\*" "$dir\b" -Force -Recurse }`,
		`if(Test-Path "$dir\other"){ Remove-Item "$dir\a" -Recurse }`,
	} {
		p, e := Compile(c, "post_install", source)
		if e == nil || p.Actions != nil {
			t.Fatalf("accepted %s", source)
		}
	}
	source := `if(Test-Path "$dir\old"){ Copy-Item "$dir\old\*" "$dir\new" -Force -Recurse; Unknown-Command; Remove-Item "$dir\old" -Recurse }`
	if p, e := Compile(Context{}, "post_install", source); e == nil || p.Actions != nil || !strings.Contains(e.Error(), "unrecognized") {
		t.Fatalf("ignored guarded remainder: %v", e)
	}
}
