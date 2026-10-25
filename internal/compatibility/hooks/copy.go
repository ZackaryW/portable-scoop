package hooks

func localCopy(c Context, n node) (operation, bool, error) {
	body, guards, err := localBody(n)
	if err != nil || !command(body, "Copy-Item") {
		return operation{}, false, nil
	}
	args, err := bind(body.command[1:], []string{"path", "destination"}, map[string]string{"-path": "path", "-literalpath": "path", "-destination": "destination"}, map[string]string{"-force": "force"})
	if err != nil {
		return operation{}, true, err
	}
	source, err := operandPath(c, args["path"], "dir", false)
	if err != nil {
		return operation{}, true, err
	}
	a := &Action{Kind: "copy-package-file", Source: source}
	if err = localTarget(c, a, args["destination"], guards, false); err != nil {
		return operation{}, true, err
	}
	return operation{action: a, reads: []string{source}, reason: "copy checked packaged default while preserving persisted data"}, true, nil
}
