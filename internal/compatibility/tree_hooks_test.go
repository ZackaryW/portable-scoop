package compatibility

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPinnedResourceManifestAndRenamedVariants(t *testing.T) {
	// Unmodified Extras bucket/potplayer.json at
	// 6c4f7709ed26a7979bad9e48f73ce549054a45b6 (version 261021).
	data, err := os.ReadFile("testdata/potplayer-261021.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, renamed := range []bool{false, true} {
		source := string(data)
		if renamed {
			source = strings.NewReplacer("PotPlayer", "OtherPlayer", "Logos", "Artwork", "AutoUpdate", "FixtureSetting").Replace(source)
		}
		m, err := Decode([]byte(source))
		if err != nil {
			t.Fatal(err)
		}
		p, err := Compile(m)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Actions) != 19 || p.Actions[0].Kind != "cleanup-package-tree" || len(p.Actions[0].Paths) != 2 {
			t.Fatalf("actions: %+v", p.Actions)
		}
		if !reflect.DeepEqual(p.Actions[0].Paths, []string{"$*", "Uninst*"}) {
			t.Fatalf("escaped cleanup paths: %+v", p.Actions[0])
		}
		for _, a := range p.Actions[1:5] {
			if a.Kind != "write-default-file" || a.Guard != "persist-missing" || !strings.HasSuffix(a.Content, "=0\r\n") {
				t.Fatalf("default: %+v", a)
			}
		}
		for i := 5; i < len(p.Actions); i += 2 {
			if p.Actions[i].Kind != "copy-package-tree" || p.Actions[i+1].Kind != "cleanup-package-tree" || p.Actions[i].Source != p.Actions[i+1].Source {
				t.Fatal("copy/cleanup order lost")
			}
		}
		again, err := Compile(p.Manifest)
		if err != nil || !reflect.DeepEqual(p.Actions, again.Actions) {
			t.Fatalf("saved manifest changed actions: %v", err)
		}
	}
}

func TestResourceManifestRejectsEveryUnsupportedRemainder(t *testing.T) {
	data, err := os.ReadFile("testdata/potplayer-261021.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		strings.Replace(string(data), "'Logos', 'Skins'", "'Logos', '../outside'", 1),
		strings.Replace(string(data), "'Logos', 'Skins'", "'Logos', \\\"$unknown\\\"", 1),
		strings.Replace(string(data), "-Force -Recurse", "-Force -Recurse; Unknown-Command", 1),
		strings.Replace(string(data), "Join-Path $dir", "Join-Path $persist_dir", 1),
	} {
		m, e := Decode([]byte(source))
		if e != nil {
			t.Fatal(e)
		}
		p, e := Compile(m)
		if e == nil || p.Actions != nil || p.Manifest != nil {
			t.Fatalf("accepted mutated resource hook: %v", e)
		}
	}
}
