package shimargs

import (
	"reflect"
	"testing"
)

func TestLiteralArguments(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{`--mode "read only" "" --key=value`, []string{"--mode", "read only", "", "--key=value"}},
		{`"say \"hello\"" 雪`, []string{`say "hello"`, "雪"}},
		{`"trailing\\" a\b`, []string{`trailing\`, `a\b`}},
		{`"a""b"`, []string{`a"b`}},
		{`& | ;`, []string{"&", "|", ";"}},
		{`--url=https://example.test/item`, []string{"--url=https://example.test/item"}},
	} {
		got, err := Parse(tc.text)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%q => %q %v want %q", tc.text, got, err, tc.want)
		}
	}
}

func TestRejectDynamicOrMalformedArguments(t *testing.T) {
	for _, text := range []string{`"unterminated`, `$dir`, `%APPDATA%`, "line\nnext", `--path=C:\host`, `\\host\share`} {
		if _, err := Parse(text); err == nil {
			t.Fatal(text)
		}
	}
}
