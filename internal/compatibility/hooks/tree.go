package hooks

import (
	"fmt"
	"path"
	"strings"
)

// Typed filename data may contain literal dollars/backticks. Script operands
// must first pass binding resolution; this does not relax unresolved syntax.
func literalTreePath(s string) bool {
	return !strings.ContainsAny(s, "[]") && safeRelative(strings.NewReplacer("$", "x", "`", "x").Replace(s))
}
func treePath(t token) (string, error) {
	if t.value == nil || t.value.root != "dir" || t.value.list != nil {
		return "", fmt.Errorf("tree paths require checked package-root values")
	}
	p := strings.TrimSuffix(t.value.text, "/")
	if !literalTreePath(p) {
		return "", fmt.Errorf("invalid literal tree path")
	}
	return p, nil
}
func treeBody(c Context, n node) (node, string, error) {
	if len(n.condition) == 0 {
		return n, "", nil
	}
	cs, err := conditions(n.condition)
	if err != nil || len(cs) != 1 || !cs[0].exists || cs[0].kind == "leaf" || len(n.body) != 1 {
		return node{}, "", fmt.Errorf("tree operation requires a single source-directory existence guard")
	}
	p, err := treePath(cs[0].path)
	return n.body[0], p, err
}
func treeOverlap(a, b string) bool { return overlaps(a, b) }
func cleanupPersistOverlap(c Context, selection string) bool {
	if !strings.HasSuffix(selection, "*") {
		return persistenceOverlap(c, selection)
	}
	prefix := strings.TrimSuffix(selection, "*")
	parent := path.Dir(prefix)
	base := strings.ToLower(path.Base(prefix))
	for _, pair := range c.Persist {
		p := strings.ToLower(strings.ReplaceAll(pair[0], `\`, "/"))
		if parent != "." {
			if overlaps(p, parent) && !strings.HasPrefix(p, strings.ToLower(parent)+"/") {
				return true
			}
			if !strings.HasPrefix(p, strings.ToLower(parent)+"/") {
				continue
			}
			p = p[len(parent)+1:]
		}
		first, _, _ := strings.Cut(p, "/")
		if strings.HasPrefix(first, base) {
			return true
		}
	}
	return false
}

// Split independent local guarded operations, preserving the complete optional
// host-import construct for its dedicated recognizer. Bindings/info are pure
// compile-time accounting; unsupported children still reject the whole hook.
func splitLocalGuards(c Context, input []node) []node {
	var out []node
	for _, n := range input {
		cs, err := conditions(n.condition)
		valid := err == nil && len(cs) == 1 && len(n.body) > 1
		if valid {
			_, a := operandPath(c, cs[0].path, "dir", false)
			_, b := operandPath(c, cs[0].path, "persist_dir", false)
			valid = a == nil || b == nil
		}
		if valid {
			for _, child := range n.body {
				if child.binding || infoNode(child) {
					out = append(out, child)
					continue
				}
				out = append(out, node{condition: n.condition, body: []node{child}, line: child.line, iteration: child.iteration})
			}
		} else {
			out = append(out, n)
		}
	}
	return out
}
