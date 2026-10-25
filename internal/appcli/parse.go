// Package appcli implements pspe's prefix-only application command surface.
package appcli

import (
	"fmt"
	"strings"

	"portscoop/internal/application"
)

type Action int

const (
	Execute Action = iota
	Help
	Version
)

func Parse(args []string) (application.Invocation, Action, error) {
	in := application.Invocation{}
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return in, Execute, fmt.Errorf("NUL is not permitted in arguments")
		}
	}
	options := true
	selectorSeen := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if options {
			switch {
			case arg == "--":
				options = false
				continue
			case arg == "--help" || arg == "-h":
				return in, Help, nil
			case arg == "--version":
				return in, Version, nil
			case arg == "--wait" || arg == "--detach":
				mode := application.Wait
				if arg == "--detach" {
					mode = application.Detach
				}
				if in.Mode != "" && in.Mode != mode {
					return in, Execute, fmt.Errorf("--wait and --detach conflict")
				}
				in.Mode = mode
				continue
			case arg == "--env" || strings.HasPrefix(arg, "--env="):
				if selectorSeen {
					return in, Execute, fmt.Errorf("--env may be supplied only once")
				}
				selectorSeen = true
				if arg == "--env" {
					if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
						return in, Execute, fmt.Errorf("--env requires an alias")
					}
					i++
					in.Selector = args[i]
				} else {
					in.Selector = strings.TrimPrefix(arg, "--env=")
				}
				if strings.TrimSpace(in.Selector) == "" {
					return in, Execute, fmt.Errorf("--env requires an alias")
				}
				continue
			case strings.HasPrefix(arg, "-"):
				return in, Execute, fmt.Errorf("unknown launcher option %q", arg)
			}
		}
		if arg == "" {
			return in, Execute, fmt.Errorf("shim name is required")
		}
		in.Shim = arg
		in.Args = append([]string{}, args[i+1:]...)
		if in.Mode == "" {
			in.Mode = application.Detach
			if len(in.Args) > 0 {
				in.Mode = application.Wait
			}
		}
		return in, Execute, nil
	}
	return in, Execute, fmt.Errorf("shim name is required; use pspe --help")
}
