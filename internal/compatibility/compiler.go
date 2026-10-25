// Package compatibility prechecks manifest fields and compiles portable actions.
// Handlers never execute scripts or access the filesystem.
package compatibility

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const Version = "portable-fields-v1"

type Manifest map[string]json.RawMessage
type Disposition string

const (
	Use     Disposition = "use"
	Rewrite Disposition = "rewrite"
	Skip    Disposition = "skip"
	Reject  Disposition = "reject"
)

type Decision struct {
	Field       string      `json:"field"`
	Disposition Disposition `json:"disposition"`
	Reason      string      `json:"reason,omitempty"`
}

// Action has no command or script payload. Paths are package-relative.
type Action struct {
	Phase string `json:"phase"`
	Kind  string `json:"kind"`
	Path  string `json:"path"`
}

type Plan struct {
	Manifest  Manifest   `json:"manifest"`
	Decisions []Decision `json:"decisions"`
	Actions   []Action   `json:"actions,omitempty"`
	Downloads []Download `json:"downloads,omitempty"`
}

type result struct {
	disposition Disposition
	reason      string
	value       json.RawMessage
	actions     []Action
	bindings    [][]string
}
type handler func(Manifest, json.RawMessage) (result, error)

var fields = map[string]handler{
	"version": version, "description": description, "homepage": homepage,
	"license": license, "url": downloadURL, "hash": downloadHash,
	"extract_dir": extractDir, "extract_to": extractTo,
	"bin": bin, "shortcuts": shortcuts, "persist": persist,
	"env_add_path": envAddPath, "env_set": envSet, "depends": depends,
	"psmodule": psmodule, "pre_install": preInstall, "post_install": postInstall,
	"pre_uninstall": preUninstall, "post_uninstall": postUninstall,
	"installer": installer, "uninstaller": uninstaller, "innosetup": innosetup,
	"cookie": cookie, "suggest": suggest, "notes": notes,
	"checkver": checkver, "autoupdate": autoupdate,
	"$schema": schema, "_comment": comment, "##": comments,
}

// Compile returns diagnostics even on rejection, but never a usable manifest or
// actions. Handler input is immutable; normalization is collected separately.
func Compile(input Manifest) (Plan, error) {
	m, err := architecture(input)
	if err != nil {
		return Plan{Decisions: []Decision{{"architecture", Reject, err.Error()}}}, err
	}
	p := Plan{Manifest: Manifest{}}
	if _, ok := input["architecture"]; ok {
		p.Decisions = append(p.Decisions, Decision{"architecture", Use, "selected Windows x64"})
	}
	keys := make([]string, 0, len(m)+3)
	for key := range m {
		keys = append(keys, key)
	}
	for _, key := range []string{"version", "url", "hash"} {
		if _, ok := m[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var failures []string
	var bindings [][]string
	for _, key := range keys {
		h, ok := fields[key]
		var r result
		var err error
		if !ok {
			err = fmt.Errorf("unsupported manifest field")
		} else {
			r, err = h(m, m[key])
		}
		d := Decision{Field: key, Disposition: r.disposition, Reason: r.reason}
		if err != nil {
			d.Disposition, d.Reason = Reject, err.Error()
			failures = append(failures, fmt.Sprintf("%s: %s", key, err))
		} else {
			if r.value != nil {
				p.Manifest[key] = append(json.RawMessage(nil), r.value...)
			}
			p.Actions = append(p.Actions, r.actions...)
			bindings = append(bindings, r.bindings...)
		}
		p.Decisions = append(p.Decisions, d)
	}
	if len(failures) == 0 {
		merged, err := mergeBindings(bindings)
		if err != nil {
			p.Decisions = append(p.Decisions, Decision{"bin", Reject, err.Error()})
			failures = append(failures, "bin: "+err.Error())
		} else if len(merged) > 0 {
			p.Manifest["bin"], _ = json.Marshal(merged)
		}
	}
	if len(failures) == 0 {
		var err error
		p.Downloads, err = compileDownloads(p.Manifest)
		if err != nil {
			p.Decisions = append(p.Decisions, Decision{"url", Reject, err.Error()})
			failures = append(failures, "url: "+err.Error())
		}
	}
	sort.SliceStable(p.Decisions, func(i, j int) bool { return p.Decisions[i].Field < p.Decisions[j].Field })
	if len(failures) != 0 {
		p.Manifest, p.Actions = nil, nil
		p.Downloads = nil
		return p, fmt.Errorf("%s: %s", Version, strings.Join(failures, "; "))
	}
	return p, nil
}

func keep(raw json.RawMessage) (result, error)  { return result{disposition: Use, value: raw}, nil }
func ignore(reason string) (result, error)      { return result{disposition: Skip, reason: reason}, nil }
func unsupported(reason string) (result, error) { return result{}, fmt.Errorf("%s", reason) }
