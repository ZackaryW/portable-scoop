package hooks

import (
	"strings"
	"testing"
)

func TestLiteralArraysLoopBindingsAndCheckedPaths(t *testing.T) {
	c := Context{Persist: [][2]string{{"a.ini", "a.ini"}, {"b.ini", "b.ini"}}}
	source := `$cfg = @(
    '[Settings]'
    'AutoUpdate=0'
)
'a.ini', 'b.ini' | ForEach-Object {
    $file = Join-Path -Path $dir -ChildPath $_
    if (-not (Test-Path "$persist_dir\$_")) {
        info "File $_ does not exist. Creating..."
        Set-Content -Path $file -Value $cfg -Encoding Ascii
    }
}`
	for _, phase := range []string{"pre_install", "post_install"} {
		plan, err := Compile(c, phase, source)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Actions) != 2 {
			t.Fatalf("actions: %+v", plan)
		}
		for i, a := range plan.Actions {
			want := []string{"a.ini", "b.ini"}[i]
			if a.Path != want || a.PersistPath != want || a.Guard != "persist-missing" || a.Content != "[Settings]\r\nAutoUpdate=0\r\n" || a.Encoding != "ascii" {
				t.Fatalf("action: %+v", a)
			}
		}
	}
}

func TestLiteralEscapeProvenance(t *testing.T) {
	source := "$name = 'literal$version'; Set-Content \"$dir\\settings\" -Value \"`$name=``quoted`\"\" -NoNewline"
	plan, err := Compile(Context{Version: "9"}, "pre_install", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Content != "$name=`quoted\"" {
		t.Fatalf("%+v", plan)
	}
	ts, err := lex("\"$dir\\`$*\"")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveToken(Context{}, bindings{}, ts[0])
	if err != nil || resolved.value.root != "dir" || resolved.value.text != "$*" {
		t.Fatalf("lost literal dollar: %+v %v", resolved, err)
	}
	for _, s := range []string{"Set-Content \"$dir\\a\" \"`n\"", `Set-Content "$dir\a" "$(Run-Code)"`, `Set-Content "$dir\a" "$unknown"`} {
		if p, e := Compile(Context{}, "pre_install", s); e == nil || p.Actions != nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestImmutableBindingsRejectDynamicRemainder(t *testing.T) {
	for _, source := range []string{
		`$dir = 'elsewhere'; Set-Content "$dir\x" 'bad'`,
		`$x='a'; $x='b'; Set-Content "$dir\x" $x`,
		`$x = Get-Content "$dir\x"; Set-Content "$dir\a" $x`,
		`$x = @('a', @('b')); Set-Content "$dir\x" $x`,
		`'a','b' | ForEach-Object { 'c','d' | ForEach-Object { Set-Content "$dir\$_" 'bad' } }`,
		`'a','../outside' | ForEach-Object { Set-Content "$dir\$_" 'bad' }`,
		`'a','b' | ForEach-Object { $_='changed'; Set-Content "$dir\$_" 'bad' }`,
		`$file = Join-Path $dir '../outside'; Set-Content $file 'bad'`,
		`$file = Join-Path 'C:\host' 'x'; Set-Content $file 'bad'`,
		`$file = Join-Path $dir $persist_dir; Set-Content $file 'bad'`,
		`$file = 'x'; Set-Content "$dir\x" $file.Length`,
		`'a','b' | ForEach-Object { Set-Content "$dir\$_" 'ok' }; Unknown-Command`,
	} {
		p, err := Compile(Context{}, "post_install", source)
		if err == nil || p.Actions != nil {
			t.Fatalf("accepted %q: %+v", source, p)
		}
	}
}

func TestArrayContentEncodingsAndLineSemantics(t *testing.T) {
	for _, tc := range []struct{ options, want string }{{"-Encoding ASCII", "a\r\nb\r\n"}, {"-Encoding utf8 -NoNewline", "ab"}, {"-Encoding unicode", "a\r\nb\r\n"}} {
		p, err := Compile(Context{}, "pre_install", `$lines='a','b'; Set-Content "$dir\file" -Value $lines `+tc.options)
		if err != nil || len(p.Actions) != 1 || p.Actions[0].Content != tc.want {
			t.Fatalf("%s: %+v %v", tc.options, p, err)
		}
	}
	if p, err := Compile(Context{}, "pre_install", `Set-Content "$dir\file" @('a','é') -Encoding Ascii`); err == nil || p.Actions != nil {
		t.Fatal("non-ASCII content accepted")
	}
}

func TestCollectionExpansionLimitsAndDiagnostics(t *testing.T) {
	list := strings.Repeat("'x',", maxCollection-1) + "'x'"
	p, err := Compile(Context{}, "pre_install", list+` | ForEach-Object { Set-Content "$dir\$_" 'a' }`)
	if err != nil || len(p.Actions) != maxCollection {
		t.Fatalf("boundary rejected: %v", err)
	}
	boundary := list + ` | ForEach-Object {` + strings.Repeat(`if (-not (Test-Path "$dir\$_")) { Set-Content "$dir\$_" 'a' };`, 16) + `}`
	if p, err := Compile(Context{}, "post_install", boundary); err != nil || len(p.Actions) != maxExpandedActions {
		t.Fatalf("action boundary: %d %v", len(p.Actions), err)
	}
	for _, s := range []string{list + `,'x' | ForEach-Object { Set-Content "$dir\$_" 'a' }`, list + ` | ForEach-Object {` + strings.Repeat(`Set-Content "$dir\$_" 'a';`, 17) + `}`, list + ` | ForEach-Object { Set-Content "$dir\$_" '` + strings.Repeat("x", 5000) + `' }`} {
		p, e := Compile(Context{}, "pre_install", s)
		if e == nil || p.Actions != nil {
			t.Fatal("unbounded expansion accepted")
		}
	}
	large := `$long='` + strings.Repeat("x", 32000) + `'; Set-Content "$dir\file" "` + strings.Repeat("$long ", 100) + `"`
	if p, e := Compile(Context{}, "pre_install", large); e == nil || p.Actions != nil {
		t.Fatal("oversized interpolation accepted")
	}
	p, err = Compile(Context{}, "post_install", "'ok','../bad' | ForEach-Object {\nSet-Content \"$dir\\$_\" 'a'\n}")
	if err == nil || p.Actions != nil || !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "iteration 2") {
		t.Fatalf("diagnostic: %v", err)
	}
}

func TestExpandedReadsStillRejectSkippedImportDependencies(t *testing.T) {
	c := Context{Persist: [][2]string{{"profile", "profile"}}}
	source := importScript("profile", "Unrelated") + `; 'settings','other' | ForEach-Object { Copy-Item "$dir\profile\$_" "$dir\$_" }`
	p, err := Compile(c, "pre_install", source)
	if err == nil || p.Actions != nil || !strings.Contains(err.Error(), "skipped host data") {
		t.Fatalf("dependency lost: %v", err)
	}
}
