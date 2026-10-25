package hooks

import "fmt"

func directory(c Context, n node) (operation, bool, error) {
	body, guards, err := localBody(n)
	if err != nil {
		return operation{}, false, nil
	}
	var p string
	if command(body, "ensure") {
		args, e := bind(body.command[1:], []string{"path"}, nil, nil)
		if e != nil {
			return operation{}, true, e
		}
		a := &Action{Kind: "ensure-directory"}
		if err = localTarget(c, a, args["path"], guards, true); err != nil {
			return operation{}, true, err
		}
		return operation{action: a, reason: "ensure checked package directory"}, true, nil
	}
	if !command(body, "New-Item") {
		return operation{}, false, nil
	}
	p, kind, err := newItem(c, body)
	if err != nil || kind != "directory" {
		return operation{}, false, nil
	}
	a := &Action{Kind: "ensure-directory"}
	if err = localTarget(c, a, token{text: "$dir/" + p}, guards, true); err != nil {
		return operation{}, true, fmt.Errorf("directory: %w", err)
	}
	return operation{action: a, reason: "ensure checked package directory"}, true, nil
}
