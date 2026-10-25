package hooks

import (
	"fmt"
	"portscoop/internal/metadata"
	"regexp"
	"strings"
)

var variableVersion = regexp.MustCompile(`(?i)\$version\b`)
var fixedVersion = regexp.MustCompile(`\A[A-Za-z0-9][A-Za-z0-9._+-]*\z`)

func safeRelative(s string) bool {
	if !metadata.RelativePath(s) || strings.ContainsAny(s, "\r\n\t\"<>|?*`$") {
		return false
	}
	for _, part := range strings.Split(strings.ReplaceAll(s, `\`, "/"), "/") {
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}
func operandPath(c Context, t token, root string, glob bool) (string, error) {
	if t.quote == '\'' {
		return "", fmt.Errorf("path variables require expanding operands")
	}
	s := strings.ReplaceAll(t.text, `\`, "/")
	prefix := "$" + root + "/"
	if !strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
		return "", fmt.Errorf("path must use %s", prefix)
	}
	s = s[len(prefix):]
	if variableVersion.MatchString(s) {
		if !fixedVersion.MatchString(c.Version) || !safeRelative(c.Version) {
			return "", fmt.Errorf("invalid version expansion")
		}
		s = variableVersion.ReplaceAllString(s, c.Version)
	}
	if glob {
		if !strings.HasSuffix(s, "/*") {
			return "", fmt.Errorf("expected directory contents wildcard")
		}
		s = strings.TrimSuffix(s, "/*")
	} else {
		s = strings.TrimSuffix(s, "/")
	}
	if !safeRelative(s) {
		return "", fmt.Errorf("unsafe or dynamic path operand")
	}
	return s, nil
}
func persisted(c Context, p string) bool {
	for _, pair := range c.Persist {
		if strings.EqualFold(strings.ReplaceAll(pair[0], `\`, "/"), p) && safeRelative(pair[1]) {
			return true
		}
	}
	return false
}
func persistenceOverlap(c Context, p string) bool {
	for _, pair := range c.Persist {
		if overlaps(pair[0], p) {
			return true
		}
	}
	return false
}

// bind accepts only explicit parameters supplied by an operation's contract.
func bind(ts []token, positions []string, values, switches map[string]string) (map[string]token, error) {
	if len(ts) >= 2 && ts[len(ts)-2].quote == 0 && ts[len(ts)-2].text == "|" && ts[len(ts)-1].quote == 0 && strings.EqualFold(ts[len(ts)-1].text, "Out-Null") {
		ts = ts[:len(ts)-2]
	}
	out := map[string]token{}
	pos := 0
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		key := ""
		if t.quote == 0 && strings.HasPrefix(t.text, "-") {
			k := strings.ToLower(t.text)
			if name, ok := switches[k]; ok {
				key = name
				t = token{text: "true", line: t.line}
			} else if name, ok := values[k]; ok {
				key = name
				i++
				if i >= len(ts) {
					return nil, fmt.Errorf("missing %s value", k)
				}
				t = ts[i]
			} else {
				return nil, fmt.Errorf("unsupported parameter %s", t.text)
			}
		} else {
			for pos < len(positions) {
				key = positions[pos]
				pos++
				if _, used := out[key]; !used {
					break
				}
				key = ""
			}
			if key == "" {
				return nil, fmt.Errorf("unexpected argument %s", t.text)
			}
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate parameter %s", key)
		}
		out[key] = t
	}
	return out, nil
}
func command(n node, name string) bool {
	return len(n.command) > 0 && n.command[0].quote == 0 && strings.EqualFold(n.command[0].text, name)
}

type testCondition struct {
	path   token
	exists bool
	kind   string
}
type conditionParser struct{ parser }

func conditions(ts []token) ([]testCondition, error) {
	for _, t := range ts {
		if t.quote == 0 && t.text == "|" {
			return nil, fmt.Errorf("pipelines cannot determine an approved condition")
		}
	}
	p := conditionParser{parser{tokens: ts}}
	cs, err := p.expr()
	if err == nil && p.pos != len(ts) {
		err = fmt.Errorf("unsupported condition remainder")
	}
	return cs, err
}
func (p *conditionParser) expr() ([]testCondition, error) {
	left, err := p.atom()
	if err != nil {
		return nil, err
	}
	for p.is("-and") {
		p.pos++
		right, err := p.atom()
		if err != nil {
			return nil, err
		}
		left = append(left, right...)
	}
	return left, nil
}
func (p *conditionParser) atom() ([]testCondition, error) {
	if p.is("!") || p.is("-not") {
		p.pos++
		cs, err := p.atom()
		if err != nil {
			return nil, err
		}
		if len(cs) != 1 {
			return nil, fmt.Errorf("negated compound conditions unsupported")
		}
		cs[0].exists = !cs[0].exists
		return cs, nil
	}
	if p.is("(") {
		p.pos++
		cs, err := p.expr()
		if err != nil {
			return nil, err
		}
		if !p.is(")") {
			return nil, fmt.Errorf("unclosed test")
		}
		p.pos++
		return cs, nil
	}
	if !p.is("Test-Path") {
		return nil, fmt.Errorf("only checked Test-Path conditions supported")
	}
	p.pos++
	start := p.pos
	for p.pos < len(p.tokens) && !p.is(")") && !p.is("-and") {
		p.pos++
	}
	args, err := bind(p.tokens[start:p.pos], []string{"path"}, map[string]string{"-path": "path", "-literalpath": "path", "-pathtype": "type"}, nil)
	if err != nil {
		return nil, err
	}
	v, ok := args["path"]
	if !ok {
		return nil, fmt.Errorf("missing test path")
	}
	kind := strings.ToLower(args["type"].text)
	if kind != "" && kind != "leaf" && kind != "container" {
		return nil, fmt.Errorf("unsupported path type")
	}
	return []testCondition{{v, true, kind}}, nil
}
