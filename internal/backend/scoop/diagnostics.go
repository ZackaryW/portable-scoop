package scoop

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"portscoop/internal/compatibility"
)

func reportCompatibility(out io.Writer, manifests map[string]FrozenManifest, verbose bool) {
	if out == nil {
		return
	}
	names := make([]string, 0, len(manifests))
	for name := range manifests {
		names = append(names, name)
	}
	sort.Strings(names)
	shown := false
	for _, name := range names {
		manifest := manifests[name]
		decisions := append([]compatibility.Decision(nil), manifest.Decisions...)
		sort.SliceStable(decisions, func(i, j int) bool {
			return diagnosticOrder(decisions[i].Field) < diagnosticOrder(decisions[j].Field)
		})
		var lines []string
		for _, decision := range decisions {
			if decision.Disposition != compatibility.Rewrite && decision.Disposition != compatibility.Skip {
				continue
			}
			if len(decision.Operations) == 0 {
				if !verbose && decision.Disposition == compatibility.Skip {
					switch decision.Field {
					case "autoupdate", "checkver", "$schema", "_comment", "##":
						continue
					}
				}
				lines = append(lines, fmt.Sprintf("  %s %s: %s", decision.Field, decision.Disposition, decision.Reason))
				continue
			}
			var details []string
			if verbose {
				for _, operation := range decision.Operations {
					details = append(details, fmt.Sprintf("    operation %d, line %d: %s - %s", operation.Position, operation.Line, operation.Disposition, operation.Reason))
				}
			} else {
				counts := map[string]int{}
				var kinds []string
				for _, action := range manifest.Actions {
					if action.Phase == decision.Field {
						if counts[action.Kind] == 0 {
							kinds = append(kinds, action.Kind)
						}
						counts[action.Kind]++
					}
				}
				for _, kind := range kinds {
					details = append(details, fmt.Sprintf("    %s: %d", actionLabel(kind), counts[kind]))
				}
				for _, operation := range decision.Operations {
					if operation.Disposition == "skip" && !operation.Informational {
						details = append(details, fmt.Sprintf("    skipped (line %d): %s", operation.Line, operation.Reason))
					}
				}
			}
			if len(details) != 0 {
				lines = append(lines, "  "+decision.Field+" "+string(decision.Disposition)+":")
				lines = append(lines, details...)
			}
		}
		if len(lines) != 0 {
			fmt.Fprintf(out, "Portable compatibility: %s\n%s\n", name, strings.Join(lines, "\n"))
			shown = true
		}
	}
	if shown && !verbose {
		fmt.Fprintln(out, "Use --verbose for individual compatibility decisions.")
	}
}

func diagnosticOrder(field string) string {
	switch field {
	case "pre_install":
		return "0"
	case "post_install":
		return "1"
	default:
		return "2" + field
	}
}

func actionLabel(kind string) string {
	switch kind {
	case "write-default-file", "ensure-persisted-file":
		return "Initialize missing settings"
	case "cleanup-package-tree":
		return "Clean package files"
	case "copy-package-tree":
		return "Refresh resource folders"
	case "copy-package-file":
		return "Copy packaged defaults"
	case "rename-package-file":
		return "Rename package files"
	case "ensure-directory":
		return "Create package folders"
	default:
		return kind
	}
}
