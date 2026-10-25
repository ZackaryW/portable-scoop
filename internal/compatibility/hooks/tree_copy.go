package hooks

import (
	"fmt"
	"strings"
)

func treeCopy(c Context, n node) (operation, bool, error) {
	body, guard, err := treeBody(c, n)
	if err != nil || !command(body, "Copy-Item") {
		return operation{}, false, nil
	}
	recursive := false
	for _, t := range body.command {
		if t.quote == 0 && strings.EqualFold(t.text, "-Recurse") {
			recursive = true
		}
	}
	if !recursive {
		return operation{}, false, nil
	}
	args, err := bind(body.command[1:], []string{"path", "destination"}, map[string]string{"-path": "path", "-destination": "destination"}, map[string]string{"-force": "force", "-recurse": "recurse"})
	if err != nil {
		return operation{}, true, err
	}
	if args["force"].text != "true" || args["recurse"].text != "true" {
		return operation{}, true, fmt.Errorf("tree copy requires explicit Force and Recurse")
	}
	source := args["path"]
	if source.value == nil || !strings.HasSuffix(source.value.text, "/*") {
		return operation{}, true, fmt.Errorf("tree copy requires literal directory contents")
	}
	v := *source.value
	v.text = strings.TrimSuffix(v.text, "/*")
	source.value = &v
	s, err := treePath(source)
	if err != nil {
		return operation{}, true, err
	}
	d, err := treePath(args["destination"])
	if err != nil {
		return operation{}, true, err
	}
	if treeOverlap(s, d) || persistenceOverlap(c, s) {
		return operation{}, true, fmt.Errorf("tree source overlaps destination or persistence")
	}
	saved := persistencePath(c, d)
	if persistenceOverlap(c, d) {
		exact := false
		for _, pair := range c.Persist {
			if strings.EqualFold(strings.ReplaceAll(pair[0], `\`, "/"), d) {
				exact = true
			}
		}
		if !exact || saved == "" {
			return operation{}, true, fmt.Errorf("tree destination requires an exact persistence mapping")
		}
	}
	a := &Action{Kind: "copy-package-tree", Source: s, Path: d, PersistPath: saved, Recursive: true, Force: true}
	if guard != "" {
		if !strings.EqualFold(s, guard) {
			return operation{}, true, fmt.Errorf("tree copy guard must name its source")
		}
		a.Guard = "source-exists"
	}
	return operation{action: a, reads: []string{s}, writes: []string{d}, reason: "refresh checked package resources with atomic file replacement"}, true, nil
}
