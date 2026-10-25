// Package hooks compiles a bounded, non-evaluating subset of lifecycle syntax.
package hooks

import (
	"fmt"
	"strings"
)

type token struct {
	text     string
	quote    byte
	line     int
	segments []stringSegment
	value    *literalValue
}
type node struct {
	command, condition []token
	loop               []token
	body               []node
	line               int
	iteration          int
	binding            bool
}

func lex(source string) ([]token, error) {
	if len(source) > 65536 {
		return nil, fmt.Errorf("hook exceeds 64 KiB")
	}
	var out []token
	line := 1
	for i := 0; i < len(source); {
		c := source[i]
		if c == ' ' || c == '\t' || c == '\r' {
			i++
			continue
		}
		if c == '#' {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if c == '`' || c == 0 {
			return nil, fmt.Errorf("line %d: escapes and NUL are unsupported", line)
		}
		if c == '\n' || c == ';' {
			out = append(out, token{text: string(c), line: line})
			if c == '\n' {
				line++
			}
			i++
			continue
		}
		if strings.ContainsRune("{}()|!,=", rune(c)) {
			out = append(out, token{text: string(c), line: line})
			i++
			continue
		}
		startLine := line
		if c == '\'' || c == '"' {
			t, next, nextLine, err := quotedToken(source, i, line)
			if err != nil {
				return nil, err
			}
			i, line = next, nextLine
			out = append(out, t)
			continue
		}
		start := i
		for i < len(source) && !strings.ContainsRune(" \t\r\n;{}()|!,=\"'`", rune(source[i])) {
			if source[i] == 0 {
				return nil, fmt.Errorf("line %d: NUL", line)
			}
			i++
		}
		out = append(out, token{text: source[start:i], line: startLine})
	}
	return out, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) is(s string) bool {
	return p.pos < len(p.tokens) && p.tokens[p.pos].quote == 0 && strings.EqualFold(p.tokens[p.pos].text, s)
}
func (p *parser) statements(depth int) ([]node, error) {
	if depth > 32 {
		return nil, fmt.Errorf("hook nesting exceeds 32")
	}
	var out []node
	for p.pos < len(p.tokens) {
		if p.is(";") || p.is("\n") {
			p.pos++
			continue
		}
		if p.is("}") {
			if depth == 0 {
				return nil, fmt.Errorf("unmatched closing block")
			}
			return out, nil
		}
		n := node{line: p.tokens[p.pos].line}
		if p.is("if") {
			p.pos++
			if !p.is("(") {
				return nil, fmt.Errorf("line %d: expected condition", n.line)
			}
			p.pos++
			start := p.pos
			nest := 1
			for p.pos < len(p.tokens) && nest > 0 {
				if p.is("(") {
					nest++
					if nest > 32 {
						return nil, fmt.Errorf("condition nesting exceeds 32")
					}
				}
				if p.is(")") {
					nest--
				}
				if nest > 0 {
					p.pos++
				}
			}
			if nest != 0 {
				return nil, fmt.Errorf("line %d: unclosed condition", n.line)
			}
			for _, t := range p.tokens[start:p.pos] {
				if t.quote == 0 && t.text == "\n" {
					continue
				}
				n.condition = append(n.condition, t)
			}
			p.pos++
			for p.is("\n") {
				p.pos++
			}
			if !p.is("{") {
				return nil, fmt.Errorf("line %d: expected block", n.line)
			}
			p.pos++
			var err error
			n.body, err = p.statements(depth + 1)
			if err != nil {
				return nil, err
			}
			if !p.is("}") {
				return nil, fmt.Errorf("line %d: unclosed block", n.line)
			}
			p.pos++
			if p.pos < len(p.tokens) && !p.is(";") && !p.is("\n") && !p.is("}") {
				return nil, fmt.Errorf("line %d: trailing block expression", n.line)
			}
		} else {
			nest := 0
			for p.pos < len(p.tokens) && (nest > 0 || (!p.is(";") && !p.is("\n") && !p.is("}"))) {
				if p.is("{") {
					count := len(n.command)
					if nest != 0 || count < 3 || n.command[count-2].text != "|" || n.command[count-2].quote != 0 || n.command[count-1].quote != 0 || !strings.EqualFold(n.command[count-1].text, "ForEach-Object") {
						return nil, fmt.Errorf("line %d: unsupported block", n.line)
					}
					n.loop = n.command[:count-2]
					n.command = nil
					p.pos++
					var err error
					n.body, err = p.statements(depth + 1)
					if err != nil {
						return nil, err
					}
					if !p.is("}") {
						return nil, fmt.Errorf("line %d: unclosed loop", n.line)
					}
					p.pos++
					if p.pos < len(p.tokens) && !p.is(";") && !p.is("\n") && !p.is("}") {
						return nil, fmt.Errorf("line %d: trailing loop expression", n.line)
					}
					break
				}
				if p.is("(") {
					nest++
					if nest > 32 {
						return nil, fmt.Errorf("expression nesting exceeds 32")
					}
				}
				if p.is(")") {
					nest--
					if nest < 0 {
						return nil, fmt.Errorf("line %d: unmatched closing expression", n.line)
					}
				}
				n.command = append(n.command, p.tokens[p.pos])
				p.pos++
			}
			if nest != 0 {
				return nil, fmt.Errorf("line %d: unclosed expression", n.line)
			}
		}
		out = append(out, n)
	}
	return out, nil
}

func parse(source string) ([]node, error) {
	ts, err := lex(source)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: ts}
	return p.statements(0)
}
