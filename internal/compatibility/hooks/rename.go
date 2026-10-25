package hooks

import (
	"fmt"
	"path"
	"strings"
)

func rename(c Context, n node) (operation, bool, error) {
	if !command(n, "Rename-Item") {
		return operation{}, false, nil
	}
	fail := func(e error) (operation, bool, error) { return operation{}, true, e }
	a, err := bind(n.command[1:], []string{"path", "name"}, map[string]string{"-path": "path", "-literalpath": "path", "-newname": "name"}, nil)
	if err != nil {
		return fail(err)
	}
	source, err := operandPath(c, a["path"], "dir", false)
	if err != nil {
		return fail(err)
	}
	name := a["name"].text
	if !safeRelative(name) || strings.ContainsAny(name, `/\`) {
		return fail(fmt.Errorf("rename requires a literal basename"))
	}
	target := path.Join(path.Dir(source), name)
	if persistenceOverlap(c, source) || persistenceOverlap(c, target) {
		return fail(fmt.Errorf("rename cannot affect declared persistence"))
	}
	return operation{action: &Action{Kind: "rename-package-file", Source: source, Path: target}, reason: "rename checked package file", reads: []string{source}}, true, nil
}
