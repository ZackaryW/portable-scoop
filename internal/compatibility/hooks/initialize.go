package hooks

import (
	"fmt"
	"strings"
)

func newItem(c Context, n node) (string, string, error) {
	if !command(n, "New-Item") {
		return "", "", fmt.Errorf("expected local New-Item")
	}
	a, err := bind(n.command[1:], []string{"path"}, map[string]string{"-path": "path", "-itemtype": "type"}, map[string]string{"-force": "force"})
	if err != nil {
		return "", "", err
	}
	p, err := operandPath(c, a["path"], "dir", false)
	if err != nil {
		return "", "", err
	}
	kind := strings.ToLower(a["type"].text)
	if kind == "" {
		kind = "file"
	}
	if kind != "file" && kind != "directory" {
		return "", "", fmt.Errorf("unsupported item type")
	}
	return p, kind, nil
}
func initialize(c Context, n node) (operation, bool, error) {
	if len(n.condition) == 0 || len(n.body) != 1 || !command(n.body[0], "New-Item") {
		return operation{}, false, nil
	}
	cs, err := conditions(n.condition)
	if err != nil || len(cs) != 1 || cs[0].exists {
		return operation{}, false, nil
	}
	p, err := operandPath(c, cs[0].path, "persist_dir", false)
	if err != nil {
		return operation{}, false, nil
	}
	target, kind, err := newItem(c, n.body[0])
	if err != nil {
		return operation{}, true, err
	}
	if kind != "file" || cs[0].kind == "container" || !strings.EqualFold(p, target) || !persisted(c, target) {
		return operation{}, true, fmt.Errorf("empty configuration requires matching persistence and file paths")
	}
	return operation{action: &Action{Kind: "ensure-persisted-file", Path: target}, reason: "initialize empty configuration while preserving persisted data"}, true, nil
}
