package appcli

import (
	"reflect"
	"testing"

	"portscoop/internal/application"
)

func TestPrefixParsing(t *testing.T) {
	for _, tc := range []struct {
		name           string
		args           []string
		selector, shim string
		tail           []string
		mode           application.Mode
		action         Action
	}{
		{"bare", []string{"editor"}, "", "editor", []string{}, application.Detach, Execute},
		{"arguments", []string{"rg", "-n", "hello", "."}, "", "rg", []string{"-n", "hello", "."}, application.Wait, Execute},
		{"wait", []string{"--wait", "python"}, "", "python", []string{}, application.Wait, Execute},
		{"detach", []string{"--detach", "editor", "file.txt"}, "", "editor", []string{"file.txt"}, application.Detach, Execute},
		{"selection", []string{"--env", "work", "rg", "--help"}, "work", "rg", []string{"--help"}, application.Wait, Execute},
		{"inline selector", []string{"--env=work", "editor"}, "work", "editor", []string{}, application.Detach, Execute},
		{"delimiter", []string{"--", "tool", "--version", "--"}, "", "tool", []string{"--version", "--"}, application.Wait, Execute},
		{"opaque tail", []string{"--env", "work", "tool", "--env", "production", "--wait", "--output=result.txt", "", "雪", `a"b`, "%!&|<>^"}, "work", "tool", []string{"--env", "production", "--wait", "--output=result.txt", "", "雪", `a"b`, "%!&|<>^"}, application.Wait, Execute},
		{"help", []string{"--help"}, "", "", nil, "", Help},
		{"short help", []string{"-h"}, "", "", nil, "", Help},
		{"version", []string{"--version"}, "", "", nil, "", Version},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, action, err := Parse(tc.args)
			if err != nil || action != tc.action || got.Selector != tc.selector || got.Shim != tc.shim || got.Mode != tc.mode || !reflect.DeepEqual(got.Args, tc.tail) {
				t.Fatalf("got %+v action=%v err=%v", got, action, err)
			}
			if len(got.Args) > 0 {
				got.Args[0] = "modified"
				if tc.args[len(tc.args)-len(tc.tail)] == "modified" {
					t.Fatal("tail aliases caller")
				}
			}
		})
	}
}

func TestInvalidPrefixNeverLaunches(t *testing.T) {
	for _, args := range [][]string{nil, {""}, {"--"}, {"--wait"}, {"--env"}, {"--env", ""}, {"--env", "--wait", "x"}, {"--env="}, {"--env=a", "--env=b", "tool"}, {"--wait", "--detach", "tool"}, {"--detach", "--wait", "tool"}, {"--unknown", "tool"}, {"tool", "NUL\x00value"}, {"--help", "\x00"}} {
		if _, _, err := Parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}
