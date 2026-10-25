package hooks

import (
	"fmt"
	"strings"
)

func literalDefault(c Context, n node) (operation, bool, error) {
	body, guards, err := localBody(n)
	if err != nil {
		return operation{}, false, nil
	}
	set := command(body, "Set-Content")
	item := command(body, "New-Item")
	if !set && !item {
		return operation{}, false, nil
	}
	values := map[string]string{"-path": "path", "-literalpath": "path", "-value": "value"}
	switches := map[string]string{"-force": "force"}
	positions := []string{"path"}
	if set {
		values["-encoding"] = "encoding"
		switches["-nonewline"] = "nonewline"
		positions = append(positions, "value")
	} else {
		values["-itemtype"] = "type"
	}
	args, err := bind(body.command[1:], positions, values, switches)
	if err != nil {
		return operation{}, true, err
	}
	if item && args["type"].text != "" && !strings.EqualFold(args["type"].text, "file") {
		return operation{}, false, nil
	}
	value := args["value"]
	if value.quote == 0 && (value.text != "" || set) {
		return operation{}, true, fmt.Errorf("default content must be a quoted literal")
	}
	if value.quote == '"' && strings.Contains(value.text, "$") {
		return operation{}, true, fmt.Errorf("expanded default content is unsupported")
	}
	encoding := strings.ToLower(args["encoding"].text)
	if encoding == "" {
		encoding = "utf8"
		if set {
			encoding = "ascii"
		}
	}
	if encoding != "ascii" && encoding != "utf8" && encoding != "unicode" {
		return operation{}, true, fmt.Errorf("unsupported default encoding")
	}
	if encoding == "ascii" {
		for _, r := range value.text {
			if r > 127 {
				return operation{}, true, fmt.Errorf("non-ASCII default content")
			}
		}
	}
	content := value.text
	if set && args["nonewline"].text != "true" {
		content += "\r\n"
	}
	a := &Action{Kind: "write-default-file", Content: content, Encoding: encoding}
	if err = localTarget(c, a, args["path"], guards, false); err != nil {
		return operation{}, true, err
	}
	return operation{action: a, reason: "initialize literal package default while preserving persisted data"}, true, nil
}
