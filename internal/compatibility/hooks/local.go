package hooks

import (
	"fmt"
	"strings"
)

func localBody(n node) (node, []testCondition, error) {
	if len(n.condition) == 0 {
		return n, nil, nil
	}
	cs, err := conditions(n.condition)
	var body []node
	for _, child := range n.body {
		if !infoNode(child) {
			body = append(body, child)
		}
	}
	if err != nil || len(cs) != 1 || cs[0].exists || len(body) != 1 {
		return node{}, nil, fmt.Errorf("local operation requires a single missing-path guard")
	}
	return body[0], cs, nil
}
func localTarget(c Context, a *Action, t token, guards []testCondition, directory bool) error {
	p, err := operandPath(c, t, "dir", false)
	if err != nil {
		return err
	}
	a.Path = p
	a.PersistPath = persistencePath(c, p)
	if len(guards) == 0 {
		return nil
	}
	g := guards[0]
	if (directory && g.kind == "leaf") || (!directory && g.kind == "container") {
		return fmt.Errorf("guard type differs from local operation")
	}
	if q, err := operandPath(c, g.path, "dir", false); err == nil && strings.EqualFold(p, q) {
		a.Guard = "package-missing"
		return nil
	}
	if q, err := operandPath(c, g.path, "persist_dir", false); err == nil && a.PersistPath != "" && strings.EqualFold(q, a.PersistPath) {
		a.Guard = "persist-missing"
		return nil
	}
	return fmt.Errorf("guard and destination must have matching ownership")
}
