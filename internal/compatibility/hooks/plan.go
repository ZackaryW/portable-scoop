package hooks

import (
	"fmt"
	"strings"
)

type Context struct {
	Version string
	Persist [][2]string
}
type Action struct {
	Phase       string   `json:"phase"`
	Kind        string   `json:"kind"`
	Path        string   `json:"path"`
	Source      string   `json:"source,omitempty"`
	Content     string   `json:"content,omitempty"`
	PersistPath string   `json:"persist_path,omitempty"`
	Encoding    string   `json:"encoding,omitempty"`
	Guard       string   `json:"guard,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	Recursive   bool     `json:"recursive,omitempty"`
	MissingOK   bool     `json:"missing_ok,omitempty"`
	Force       bool     `json:"force,omitempty"`
}
type Decision struct {
	Phase       string `json:"phase"`
	Position    int    `json:"position"`
	Line        int    `json:"line"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
}
type Plan struct {
	Actions                      []Action
	Decisions                    []Decision
	Reads, Writes, SkippedWrites []string
}
type operation struct {
	action               *Action
	reason               string
	reads, skippedWrites []string
	writes               []string
}
type recognizer func(Context, node) (operation, bool, error)

var operations = []recognizer{structural, rename, initialize, optionalImport, cleanup, treeCopy, localCopy, directory, literalDefault}

func Compile(c Context, phase, source string) (Plan, error) {
	return compile(c, phase, source, operations)
}
func compile(c Context, phase, source string, registry []recognizer) (Plan, error) {
	var plan Plan
	fail := func(pos, line int, reason string) (Plan, error) {
		plan.Actions = nil
		plan.Decisions = append(plan.Decisions, Decision{phase, pos, line, "reject", reason})
		return plan, fmt.Errorf("%s operation %d line %d: %s", phase, pos, line, reason)
	}
	if phase != "pre_install" && phase != "post_install" {
		return fail(0, 0, "unsupported lifecycle phase")
	}
	nodes, err := parse(source)
	if err != nil {
		return fail(0, 0, err.Error())
	}
	nodes, err = (&expansion{context: c}).nodes(nodes, bindings{}, false, 0)
	if err != nil {
		return fail(0, 0, err.Error())
	}
	nodes = splitLocalGuards(c, nodes)
	var accepted []operation
	for i, n := range nodes {
		found := false
		for _, recognize := range registry {
			op, match, err := recognize(c, n)
			if !match {
				continue
			}
			found = true
			if err != nil {
				if n.iteration > 0 {
					err = fmt.Errorf("iteration %d: %w", n.iteration, err)
				}
				return fail(i+1, n.line, err.Error())
			}
			disposition := "skip"
			if op.action != nil {
				disposition = "rewrite"
				op.action.Phase = phase
				plan.Actions = append(plan.Actions, *op.action)
				if len(plan.Actions) > maxExpandedActions {
					return fail(i+1, n.line, "expanded actions exceed 4096")
				}
			}
			plan.Decisions = append(plan.Decisions, Decision{phase, i + 1, n.line, disposition, op.reason})
			accepted = append(accepted, op)
			break
		}
		if !found {
			return fail(i+1, n.line, fmt.Sprintf("unrecognized operation (iteration %d); complete hook required", n.iteration))
		}
	}
	for i, op := range accepted {
		plan.Reads = append(plan.Reads, op.reads...)
		plan.Writes = append(plan.Writes, op.writes...)
		plan.SkippedWrites = append(plan.SkippedWrites, op.skippedWrites...)
		for _, read := range op.reads {
			for _, other := range accepted {
				for _, write := range other.skippedWrites {
					if overlaps(read, write) {
						return fail(i+1, nodes[i].line, "required operation depends on skipped host data")
					}
				}
			}
		}
	}
	return plan, nil
}

// CheckDependencies also accounts for dependencies between lifecycle phases.
func CheckDependencies(plans ...Plan) error {
	for _, p := range plans {
		for _, read := range p.Reads {
			for _, q := range plans {
				for _, write := range q.SkippedWrites {
					if overlaps(read, write) {
						return fmt.Errorf("required operation depends on skipped host data")
					}
				}
			}
		}
	}
	return nil
}
func overlaps(a, b string) bool {
	a = strings.ToLower(strings.ReplaceAll(a, `\`, "/"))
	b = strings.ToLower(strings.ReplaceAll(b, `\`, "/"))
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
