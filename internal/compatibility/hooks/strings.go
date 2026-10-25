package hooks

import (
	"fmt"
	"strings"
)

// Literal and variable segments remain distinct until checked binding. In
// particular, an escaped dollar sign can never become a second interpolation.
type stringSegment struct {
	text     string
	variable bool
}
type literalValue struct {
	root, text string
	list       []literalValue
}

func variableByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == ':'
}
func quotedToken(source string, start, line int) (token, int, int, error) {
	q := source[start]
	t := token{quote: q, line: line}
	i := start + 1
	var literal, all strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			t.segments = append(t.segments, stringSegment{text: literal.String()})
			literal.Reset()
		}
	}
	for i < len(source) {
		c := source[i]
		i++
		if c == q {
			if q == '\'' && i < len(source) && source[i] == q {
				i++
				literal.WriteByte(q)
				all.WriteByte(q)
				continue
			}
			flush()
			t.text = all.String()
			if i < len(source) && !strings.ContainsRune(" \t\r\n;{}()|!,=", rune(source[i])) {
				return token{}, i, line, fmt.Errorf("line %d: concatenated quoted expressions unsupported", line)
			}
			return t, i, line, nil
		}
		if c == 0 {
			return token{}, i, line, fmt.Errorf("line %d: NUL", line)
		}
		if c == '`' && q == '"' {
			if i >= len(source) || !strings.ContainsRune("$`\"", rune(source[i])) {
				return token{}, i, line, fmt.Errorf("line %d: unsupported quoted escape", line)
			}
			c = source[i]
			i++
			literal.WriteByte(c)
			all.WriteByte(c)
			continue
		}
		if c == '$' && q == '"' {
			flush()
			vstart := i
			for i < len(source) && variableByte(source[i]) {
				i++
			}
			if i == vstart {
				return token{}, i, line, fmt.Errorf("line %d: unsupported interpolation", line)
			}
			name := source[vstart:i]
			t.segments = append(t.segments, stringSegment{text: name, variable: true})
			all.WriteString("$" + name)
			continue
		}
		if c == '\n' {
			line++
		}
		literal.WriteByte(c)
		all.WriteByte(c)
	}
	return token{}, i, line, fmt.Errorf("line %d: unterminated string", t.line)
}
