package hooks

import (
	"fmt"
	"regexp"
	"strings"
)

const maxCollection = 256
const maxExpandedNodes = 16384
const maxExpandedActions = 4096
const maxLiteralBytes = 1 << 20

var localName = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*$`)

type bindings map[string]literalValue
type expansion struct {
	context      Context
	count, bytes int
}

func cloneBindings(in bindings) bindings {
	out := bindings{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func variable(c Context, scope bindings, name string) (literalValue, error) {
	name = strings.ToLower(name)
	if v, ok := scope[name]; ok {
		return v, nil
	}
	switch name {
	case "dir", "persist_dir", "env:appdata":
		return literalValue{root: name}, nil
	case "version":
		if fixedVersion.MatchString(c.Version) && safeRelative(c.Version) {
			return literalValue{text: c.Version}, nil
		}
	}
	return literalValue{}, fmt.Errorf("unresolved variable $%s", name)
}

func resolveToken(c Context, scope bindings, t token) (token, error) {
	if t.value != nil {
		return t, nil
	}
	if t.quote == 0 && localName.MatchString(t.text) {
		v, err := variable(c, scope, t.text[1:])
		if err != nil {
			return t, err
		}
		return valueToken(t, v), nil
	}
	segments := t.segments
	if t.quote == 0 {
		if !strings.Contains(t.text, "$") {
			return t, nil
		}
		end := 1
		for end < len(t.text) && variableByte(t.text[end]) {
			end++
		}
		if !strings.HasPrefix(t.text, "$") || end == len(t.text) || !strings.ContainsRune(`/\`, rune(t.text[end])) {
			return t, fmt.Errorf("bare variable expressions require a literal path separator")
		}
		// Bare root paths retain their existing supported spelling, but are
		// resolved with the same provenance rules as double-quoted operands.
		parsed, _, _, err := quotedToken(`"`+t.text+`"`, 0, t.line)
		if err != nil {
			return t, err
		}
		segments = parsed.segments
	}
	var text strings.Builder
	root := ""
	for _, segment := range segments {
		if !segment.variable {
			if text.Len()+len(segment.text) > maxLiteralBytes {
				return t, fmt.Errorf("expanded literals exceed 1 MiB")
			}
			text.WriteString(segment.text)
			continue
		}
		v, err := variable(c, scope, segment.text)
		if err != nil {
			return t, err
		}
		if v.list != nil {
			return t, fmt.Errorf("collections cannot be interpolated")
		}
		if text.Len()+len(v.text)+1 > maxLiteralBytes {
			return t, fmt.Errorf("expanded literals exceed 1 MiB")
		}
		if v.root != "" {
			if root != "" || text.Len() != 0 {
				return t, fmt.Errorf("path root must be the first segment")
			}
			root = v.root
			if v.text != "" {
				text.WriteString("/" + v.text)
			}
		} else {
			text.WriteString(v.text)
		}
	}
	value := literalValue{root: root, text: text.String()}
	if root != "" {
		value.text = strings.ReplaceAll(value.text, `\`, "/")
		if value.text != "" && !strings.HasPrefix(value.text, "/") {
			return t, fmt.Errorf("root requires a path separator")
		}
		value.text = strings.TrimPrefix(value.text, "/")
	}
	return valueToken(t, value), nil
}

func valueToken(t token, v literalValue) token {
	t.value = &v
	if v.list == nil {
		if v.root == "" {
			t.text = v.text
			t.quote = '\''
		} else {
			t.text = "$" + v.root
			if v.text != "" {
				t.text += "/" + v.text
			}
			t.quote = '"'
		}
	}
	return t
}

func resolvedValue(c Context, scope bindings, t token) (literalValue, error) {
	t, err := resolveToken(c, scope, t)
	if err != nil {
		return literalValue{}, err
	}
	if t.value == nil {
		return literalValue{}, fmt.Errorf("expected a literal or immutable binding")
	}
	return *t.value, nil
}

func expression(c Context, scope bindings, ts []token) (literalValue, error) {
	if len(ts) == 0 {
		return literalValue{}, fmt.Errorf("empty expression")
	}
	array := len(ts) >= 3 && ts[0].quote == 0 && ts[0].text == "@" && ts[1].text == "(" && ts[len(ts)-1].text == ")"
	if array {
		ts = ts[2 : len(ts)-1]
	}
	if !array && len(ts) >= 2 && ts[0].text == "(" && ts[len(ts)-1].text == ")" {
		ts = ts[1 : len(ts)-1]
	}
	if !array && len(ts) > 0 && ts[0].quote == 0 && strings.EqualFold(ts[0].text, "Join-Path") {
		args, err := normalize(c, scope, ts[1:])
		if err != nil {
			return literalValue{}, err
		}
		bound, err := bind(args, []string{"path", "child"}, map[string]string{"-path": "path", "-childpath": "child"}, nil)
		if err != nil {
			return literalValue{}, err
		}
		parent, err := resolvedValue(c, scope, bound["path"])
		if err != nil {
			return literalValue{}, err
		}
		child, err := resolvedValue(c, scope, bound["child"])
		if err != nil {
			return literalValue{}, err
		}
		if parent.root == "" || parent.list != nil || child.root != "" || child.list != nil || !literalTreePath(child.text) || (parent.text != "" && !literalTreePath(parent.text)) {
			return literalValue{}, fmt.Errorf("Join-Path requires a checked root and literal relative child")
		}
		if len(parent.text)+len(child.text)+1 > maxLiteralBytes {
			return literalValue{}, fmt.Errorf("expanded literals exceed 1 MiB")
		}
		if parent.text != "" {
			parent.text += "/"
		}
		parent.text += strings.ReplaceAll(child.text, `\`, "/")
		return parent, nil
	}
	list := []literalValue{}
	needValue := true
	comma := false
	for _, t := range ts {
		if t.quote == 0 && (t.text == "," || (array && t.text == "\n")) {
			if t.text == "," {
				if needValue {
					return literalValue{}, fmt.Errorf("empty list element")
				}
				comma = true
				needValue = true
			} else if !comma {
				needValue = true
			}
			continue
		}
		if !needValue {
			return literalValue{}, fmt.Errorf("concatenated list expressions unsupported")
		}
		v, err := resolvedValue(c, scope, t)
		if err != nil {
			return literalValue{}, err
		}
		if v.list != nil {
			if !array && len(ts) == 1 {
				return v, nil
			}
			return literalValue{}, fmt.Errorf("nested collections unsupported")
		}
		list = append(list, v)
		if len(list) > maxCollection {
			return literalValue{}, fmt.Errorf("collection exceeds %d elements", maxCollection)
		}
		needValue = false
		comma = false
	}
	if comma || len(list) == 0 {
		return literalValue{}, fmt.Errorf("incomplete collection")
	}
	if !array && len(list) == 1 {
		return list[0], nil
	}
	return literalValue{list: list}, nil
}

func normalize(c Context, scope bindings, ts []token) ([]token, error) {
	var out []token
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		end := i
		if t.quote == 0 && (t.text == "@" || (t.text == "(" && i+1 < len(ts) && strings.EqualFold(ts[i+1].text, "Join-Path"))) {
			nest := 0
			found := false
			for j := i; j < len(ts); j++ {
				if ts[j].quote == 0 {
					if ts[j].text == "(" {
						nest++
					}
					if ts[j].text == ")" {
						nest--
						if nest == 0 {
							end = j
							found = true
							break
						}
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("unclosed value expression")
			}
		} else if i+1 < len(ts) && ts[i+1].quote == 0 && ts[i+1].text == "," {
			end = i + 1
			for end < len(ts) && ts[end].quote == 0 && ts[end].text == "," {
				end += 2
			}
			end--
			if end >= len(ts) {
				return nil, fmt.Errorf("incomplete list")
			}
		}
		if end > i {
			v, err := expression(c, scope, ts[i:end+1])
			if err != nil {
				return nil, err
			}
			t.value = &v
			i = end
		} else {
			var err error
			t, err = resolveToken(c, scope, t)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, t)
	}
	return out, nil
}

func infoNode(n node) bool {
	if !command(n, "info") && !command(n, "Write-Host") {
		return false
	}
	return len(n.command) == 2 && n.command[1].value != nil && n.command[1].value.root == "" && n.command[1].value.list == nil
}

func (e *expansion) nodes(input []node, scope bindings, inLoop bool, iteration int) ([]node, error) {
	var out []node
	for _, n := range input {
		n.iteration = iteration
		fail := func(err error) ([]node, error) {
			return nil, fmt.Errorf("line %d iteration %d: %w", n.line, iteration, err)
		}
		if len(n.loop) > 0 {
			if inLoop {
				return fail(fmt.Errorf("nested loops unsupported"))
			}
			v, err := expression(e.context, scope, n.loop)
			if err != nil {
				return fail(err)
			}
			if v.list == nil {
				v.list = []literalValue{v}
			}
			for i, item := range v.list {
				if item.root != "" || item.list != nil {
					return fail(fmt.Errorf("loop requires literal strings"))
				}
				child := cloneBindings(scope)
				child["_"] = item
				expanded, err := e.nodes(n.body, child, true, i+1)
				if err != nil {
					return nil, err
				}
				out = append(out, expanded...)
			}
			continue
		}
		if len(n.command) >= 2 && n.command[0].quote == 0 && localName.MatchString(n.command[0].text) && n.command[1].quote == 0 && n.command[1].text == "=" {
			name := strings.ToLower(n.command[0].text[1:])
			if _, exists := scope[name]; exists || name == "dir" || name == "persist_dir" || name == "version" || name == "_" {
				return fail(fmt.Errorf("binding $%s is immutable", name))
			}
			v, err := expression(e.context, scope, n.command[2:])
			if err != nil {
				return fail(err)
			}
			scope[name] = v
			// Keep an accounting node for the non-executable binding.
			n.binding = true
			n.command = []token{{line: n.line, value: &v}}
		} else {
			var err error
			n.command, err = normalize(e.context, scope, n.command)
			if err != nil {
				return fail(err)
			}
			n.condition, err = normalize(e.context, scope, n.condition)
			if err != nil {
				return fail(err)
			}
			if len(n.body) > 0 {
				n.body, err = e.nodes(n.body, cloneBindings(scope), inLoop, iteration)
				if err != nil {
					return nil, err
				}
			}
		}
		e.count++
		if e.count > maxExpandedNodes {
			return fail(fmt.Errorf("expanded operations exceed %d", maxExpandedNodes))
		}
		for _, t := range append(append([]token{}, n.command...), n.condition...) {
			if t.value != nil {
				e.bytes += valueBytes(*t.value)
				if e.bytes > maxLiteralBytes {
					return fail(fmt.Errorf("expanded literals exceed 1 MiB"))
				}
			}
		}
		out = append(out, n)
	}
	return out, nil
}
func valueBytes(v literalValue) int {
	size := len(v.text)
	for _, item := range v.list {
		size += len(item.text)
	}
	return size
}

func structural(_ Context, n node) (operation, bool, error) {
	if n.binding {
		return operation{reason: "resolve immutable literal binding"}, true, nil
	}
	if infoNode(n) {
		return operation{reason: "recognize literal informational output"}, true, nil
	}
	return operation{}, false, nil
}
