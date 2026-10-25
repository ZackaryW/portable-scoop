package registry

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"portscoop/internal/environment"
)

func TestRegistryContract(t *testing.T) {
	i, _ := environment.NewIdentity("work")
	d := Empty()
	d.Entries["work"] = Entry{ID: i.ID, Path: filepath.Join(t.TempDir(), "work"), RelativePath: "tools/work"}
	d.DefaultID = i.ID
	data, _ := json.Marshal(d)
	if _, err := Decode(data); err != nil {
		t.Fatal(err)
	}
	if alias, err := Alias("WORK"); err != nil || alias != "work" {
		t.Fatal("alias is not case insensitive")
	}
	for name, change := range map[string]func(*Document){
		"version":       func(d *Document) { d.SchemaVersion++ },
		"default":       func(d *Document) { d.DefaultID = "missing" },
		"relative-root": func(d *Document) { e := d.Entries["work"]; e.Path = "relative"; d.Entries["work"] = e },
		"traversal":     func(d *Document) { e := d.Entries["work"]; e.RelativePath = `..\host`; d.Entries["work"] = e },
		"alias":         func(d *Document) { d.Entries["WORK"] = d.Entries["work"] },
	} {
		t.Run(name, func(t *testing.T) {
			copy, _ := Decode(data)
			change(&copy)
			raw, _ := json.Marshal(copy)
			if _, err := Decode(raw); err == nil {
				t.Fatal("accepted invalid registry")
			}
		})
	}
}
