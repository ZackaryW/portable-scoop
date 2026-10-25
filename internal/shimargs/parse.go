// Package shimargs parses literal native Windows shim arguments without a shell.
package shimargs

import (
	"fmt"
	"regexp"
	"strings"
)

var drivePath = regexp.MustCompile(`(^|[= \t])[A-Za-z]:[\\/]`)

// Parse implements Windows backslash/quote argument rules. Dynamic expressions
// and absolute paths are excluded from fixed declarations to avoid baking host
// locations or environment expansion into a portable shim.
func Parse(text string) ([]string, error) {
	if len(text) > 32767 || strings.ContainsAny(text, "\x00\r\n`$%") {
		return nil, fmt.Errorf("fixed arguments must be bounded literal values without expansion")
	}
	var args []string
	for i := 0; i < len(text); {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i == len(text) {
			break
		}
		var b strings.Builder
		quoted := false
		for i < len(text) {
			if !quoted && (text[i] == ' ' || text[i] == '\t') {
				break
			}
			n := 0
			for i < len(text) && text[i] == '\\' {
				n++
				i++
			}
			if i < len(text) && text[i] == '"' {
				b.WriteString(strings.Repeat(`\`, n/2))
				if n%2 == 1 {
					b.WriteByte('"')
				} else if quoted && i+1 < len(text) && text[i+1] == '"' {
					b.WriteByte('"')
					i++
				} else {
					quoted = !quoted
				}
				i++
				continue
			}
			b.WriteString(strings.Repeat(`\`, n))
			if i == len(text) {
				break
			}
			if !quoted && (text[i] == ' ' || text[i] == '\t') {
				break
			}
			b.WriteByte(text[i])
			i++
		}
		if quoted {
			return nil, fmt.Errorf("unbalanced fixed argument quotes")
		}
		value := b.String()
		// Reject drive/UNC paths even in --option=C:\... forms.
		if drivePath.MatchString(value) || strings.Contains(value, `\\`) {
			return nil, fmt.Errorf("fixed arguments cannot embed absolute host paths")
		}
		args = append(args, value)
	}
	return args, nil
}
