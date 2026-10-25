package hooks

import (
	"fmt"
	"strings"
)

func optionalImport(c Context, n node) (operation, bool, error) {
	if len(n.condition) == 0 {
		return operation{}, false, nil
	}
	cs, err := conditions(n.condition)
	if err != nil || len(cs) != 2 {
		return operation{}, false, nil
	}
	var dest, host string
	for _, test := range cs {
		if test.exists {
			if test.kind == "leaf" {
				return operation{}, false, nil
			}
			host, err = operandPath(c, test.path, "env:appdata", false)
		} else {
			if test.kind != "" {
				return operation{}, false, nil
			}
			dest, err = operandPath(c, test.path, "dir", true)
		}
		if err != nil {
			return operation{}, false, nil
		}
	}
	if dest == "" || host == "" {
		return operation{}, false, nil
	}
	fail := func(reason string) (operation, bool, error) {
		return operation{}, true, fmt.Errorf("optional import: %s", reason)
	}
	if !persisted(c, dest) {
		return fail("destination must have declared persistence")
	}
	body := n.body
	if len(body) == 2 {
		guard := body[0]
		tests, e := conditions(guard.condition)
		if e != nil || len(tests) != 1 || tests[0].exists || tests[0].kind != "" || len(guard.body) != 1 {
			return fail("unrecognized directory creation guard")
		}
		p, e := operandPath(c, tests[0].path, "dir", false)
		if e != nil || !strings.EqualFold(p, dest) {
			return fail("directory guard mismatch")
		}
		p, kind, e := newItem(c, guard.body[0])
		if e != nil || kind != "directory" || !strings.EqualFold(p, dest) {
			return fail("directory creation mismatch")
		}
		body = body[1:]
	}
	if len(body) != 1 || !command(body[0], "Copy-Item") {
		return fail("complete guarded copy construct required")
	}
	a, e := bind(body[0].command[1:], []string{"path", "destination"}, map[string]string{"-path": "path", "-destination": "destination"}, map[string]string{"-recurse": "recurse", "-force": "force"})
	if e != nil {
		return fail(e.Error())
	}
	s, e := operandPath(c, a["path"], "env:appdata", true)
	if e != nil || !strings.EqualFold(s, host) {
		return fail("source does not match existence guard")
	}
	d, e := operandPath(c, a["destination"], "dir", false)
	if e != nil || !strings.EqualFold(d, dest) || a["recurse"].text != "true" {
		return fail("destination or recursive copy mismatch")
	}
	return operation{reason: "skip optional host AppData import into " + dest + "; retain fresh or existing portable profile", skippedWrites: []string{dest}}, true, nil
}
