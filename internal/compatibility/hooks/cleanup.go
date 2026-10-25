package hooks

import (
	"fmt"
	"strings"
)

func cleanup(c Context, n node) (operation, bool, error) {
	body, guard, err := treeBody(c, n)
	if err != nil || !command(body, "Remove-Item") {
		return operation{}, false, nil
	}
	args, err := bind(body.command[1:], []string{"path"}, map[string]string{"-path": "path", "-literalpath": "literal", "-erroraction": "error"}, map[string]string{"-force": "force", "-recurse": "recurse"})
	if err != nil {
		return operation{}, true, err
	}
	t, ok := args["path"]
	literal, hasLiteral := args["literal"]
	if ok && hasLiteral {
		return operation{}, true, fmt.Errorf("duplicate cleanup selection")
	}
	if hasLiteral {
		t = literal
	}
	if t.value == nil {
		return operation{}, true, fmt.Errorf("cleanup requires literal checked paths")
	}
	values := t.value.list
	if values == nil {
		values = []literalValue{*t.value}
	}
	a := &Action{Kind: "cleanup-package-tree", Recursive: args["recurse"].text == "true", Force: args["force"].text == "true"}
	switch strings.ToLower(args["error"].text) {
	case "silentlycontinue":
		a.MissingOK = true
	case "", "stop":
	default:
		return operation{}, true, fmt.Errorf("unsupported cleanup error policy")
	}
	for _, v := range values {
		if v.root != "dir" || v.list != nil {
			return operation{}, true, fmt.Errorf("cleanup must remain package-local")
		}
		p := strings.TrimSuffix(v.text, "/")
		checked := p
		if strings.HasSuffix(p, "*") && !hasLiteral {
			checked = strings.TrimSuffix(p, "*")
		}
		if !literalTreePath(checked) || strings.HasSuffix(checked, "/") {
			return operation{}, true, fmt.Errorf("cleanup requires literal paths or nonempty terminal prefixes")
		}
		if cleanupPersistOverlap(c, p) {
			return operation{}, true, fmt.Errorf("cleanup overlaps declared persistence")
		}
		a.Paths = append(a.Paths, p)
	}
	if guard != "" {
		if len(a.Paths) != 1 || !strings.EqualFold(a.Paths[0], guard) {
			return operation{}, true, fmt.Errorf("cleanup guard must name its exact selection")
		}
		a.Guard = "source-exists"
		a.Source = guard
	}
	return operation{action: a, reads: append([]string{}, a.Paths...), writes: append([]string{}, a.Paths...), reason: "remove fully checked package selections without touching persistence"}, true, nil
}
