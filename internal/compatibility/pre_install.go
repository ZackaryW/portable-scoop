package compatibility

import (
	"encoding/json"
	"fmt"
	"portscoop/internal/compatibility/hooks"
	"strings"
)

func preInstall(m Manifest, raw json.RawMessage) (result, error) {
	return compileHook(m, raw, "pre_install")
}
func compileHook(m Manifest, raw json.RawMessage, phase string) (result, error) {
	lines, err := stringsValue(raw)
	if err != nil {
		return result{}, err
	}
	version, _ := stringValue(m["version"])
	c := hooks.Context{Version: version}
	if entries, err := tuples(m["persist"]); err == nil {
		for _, entry := range entries {
			if len(entry) > 0 {
				pair := [2]string{entry[0], entry[0]}
				if len(entry) > 1 {
					pair[1] = entry[1]
				}
				c.Persist = append(c.Persist, pair)
			}
		}
	}
	plan, err := hooks.Compile(c, phase, strings.Join(lines, "\n"))
	if err != nil {
		return result{}, err
	}
	reasons := []string{}
	for _, d := range plan.Decisions {
		reasons = append(reasons, fmt.Sprintf("operation %d line %d: %s (%s)", d.Position, d.Line, d.Disposition, d.Reason))
	}
	// Retain declarations so saved-manifest recompilation reproduces decisions.
	disposition := Rewrite
	if len(plan.Actions) == 0 {
		disposition = Skip
	}
	return result{disposition: disposition, reason: strings.Join(reasons, "; "), value: raw, actions: plan.Actions, hookPlan: plan}, nil
}
