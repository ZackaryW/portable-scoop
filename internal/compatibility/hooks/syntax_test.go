package hooks

import (
	"reflect"
	"testing"
)

func TestSyntaxBoundaries(t *testing.T) {
	nodes, err := parse("# leading comment\nRename-Item \"$dir\\a.exe\" 'b.exe'; Rename-Item \"$dir\\b.exe\" 'c.exe' # tail\n")
	if err != nil || len(nodes) != 2 || nodes[0].command[1].text != `$dir\a.exe` {
		t.Fatalf("%+v %v", nodes, err)
	}
	nodes, err = parse(`if (!(Test-Path "$dir\data\*") -and (Test-Path "$env:Appdata\X")) { if (!(Test-Path "$dir\data")) { New-Item "$dir\data" -ItemType Directory | Out-Null }; Copy-Item "$env:Appdata\X\*" "$dir\data\" -Recurse -Force }`)
	if err != nil || len(nodes) != 1 || len(nodes[0].body) != 2 {
		t.Fatalf("%+v %v", nodes, err)
	}
	for _, s := range []string{`'unterminated`, `if (x) { Foo`, `Foo }`, `if (x) { Foo } else { Bar }`, "Foo `\n Bar", `"a"suffix`, `{ Foo }`} {
		if _, err := parse(s); err == nil {
			t.Fatalf("accepted malformed syntax %q", s)
		}
	}
}
func TestCompilerAccountingAndDiagnostics(t *testing.T) {
	unit := func(_ Context, n node) (operation, bool, error) {
		if len(n.command) == 1 && n.command[0].quote == 0 && n.command[0].text == "Known" {
			return operation{action: &Action{Kind: "test"}, reason: "known"}, true, nil
		}
		return operation{}, false, nil
	}
	for _, phase := range []string{"pre_install", "post_install"} {
		p, err := compile(Context{}, phase, "Known;Known", []recognizer{unit})
		if err != nil || len(p.Actions) != 2 || p.Actions[1].Phase != phase {
			t.Fatalf("%+v %v", p, err)
		}
		again, _ := compile(Context{}, phase, "Known;Known", []recognizer{unit})
		if !reflect.DeepEqual(p, again) {
			t.Fatal("unstable plan")
		}
		for _, s := range []string{`Known; Unknown`, `"Known"`, `Known $(Unknown)`, `if (Unknown) { Known }`} {
			p, err := compile(Context{}, phase, s, []recognizer{unit})
			if err == nil || p.Actions != nil || p.Decisions[len(p.Decisions)-1].Disposition != "reject" {
				t.Fatalf("accepted %q", s)
			}
		}
	}
}
