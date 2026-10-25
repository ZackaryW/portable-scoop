package selfupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
	"portscoop/internal/registry"
	"portscoop/internal/resolver"
)

// HostMaintenance has explicit registry/discovery boundaries. Direct Store
// invocation never reads the user registry or resolves other installations.
type HostMaintenance struct {
	Service      Service
	ReadRegistry func() (registry.Document, error)
	Resolve      func(context.Context, resolver.Selection) (resolver.Target, error)
}

type pinnedRelease struct{ release Release }

func (p pinnedRelease) Resolve(context.Context, string) (Release, error) { return p.release, nil }

func (h HostMaintenance) Run(ctx context.Context, exe string, o Options) (Result, error) {
	if err := validateOptions(o); err != nil {
		return Result{}, err
	}
	s := h.Service
	host, err := s.Files.Select(exe, !o.Status && !o.Recover)
	if err != nil {
		return Result{}, err
	}
	if host.Kind != "host" {
		return s.Run(ctx, exe, o)
	}
	if !o.Status && !o.Recover {
		if !semver.IsValid(normalize(host.Version)) && o.Version == "" {
			return Result{}, fmt.Errorf("development version %s requires an explicit --version", host.Version)
		}
		r, err := s.Discovery.Resolve(ctx, o.Version)
		if err != nil {
			return Result{}, err
		}
		if o.Version != "" && semver.IsValid(normalize(host.Version)) && semver.Compare(normalize(host.Version), r.Tag) > 0 {
			return Result{}, fmt.Errorf("downgrading %s to %s is unsupported", host.Version, r.Tag)
		}
		s.Discovery = pinnedRelease{r}
	}
	cache := &cachedDownloads{source: s.Downloads}
	defer cache.close()
	s.Downloads = cache
	// Protect the current controller while preparing/committing Store updates.
	// Read-only queries deliberately do not create Host controls or leases.
	var lease Lease
	if !o.Check && !o.Status && !o.Recover {
		host, err = s.Files.Ensure(host)
		if err != nil {
			return Result{}, err
		}
		lease, err = s.Locks.Acquire(ctx, host, "runtime", false)
		if err != nil {
			return Result{}, err
		}
		defer lease.Close()
		if err = s.Files.Pending(host); err == nil {
			return Result{}, fmt.Errorf("Host update is pending; use --status or --recover")
		} else if !os.IsNotExist(err) {
			return Result{}, err
		}
	}
	lines := []string{}
	code := 0
	appendResult := func(label string, r Result, err error) {
		if err != nil {
			r = Result{2, err.Error()}
		}
		lines = append(lines, label+": "+r.Message)
		if r.Code == 2 || (r.Code != 0 && r.Code != 3) {
			code = 2
		} else if r.Code == 3 && code == 0 {
			code = 3
		}
	}
	doc, registryErr := h.ReadRegistry()
	if registryErr != nil {
		appendResult("Registry", Result{}, registryErr)
	} else {
		aliases := make([]string, 0, len(doc.Entries))
		for a := range doc.Entries {
			aliases = append(aliases, a)
		}
		sort.Strings(aliases)
		groups := map[string][]string{}
		order := []string{}
		for _, a := range aliases {
			id := doc.Entries[a].ID
			if len(groups[id]) == 0 {
				order = append(order, id)
			}
			groups[id] = append(groups[id], a)
		}
		for _, id := range order {
			if ctx.Err() != nil {
				return Result{130, strings.Join(append(lines, "Cancelled; remaining installations were not updated."), "\n")}, nil
			}
			names := groups[id]
			hints := []registry.Entry{}
			for _, a := range names {
				hints = append(hints, doc.Entries[a])
			}
			label := "Store " + strings.Join(names, ",") + " [" + id + "]"
			target, err := h.Resolve(ctx, resolver.Selection{ID: id, Hints: hints})
			if err != nil {
				appendResult(label+" ("+hints[0].Path+")", Result{}, err)
				continue
			}
			label += " (" + target.Paths.Root + ")"
			child := s
			child.controller = &host
			child.selectedID = id
			path := filepath.Join(target.Paths.Root, "psp.exe")
			var result Result
			if o.Recover {
				result, err = recoverIfPending(ctx, child, path)
			} else {
				result, err = child.Run(ctx, path, o)
			}
			appendResult(label, result, err)
		}
	}
	if lease != nil {
		lease.Close()
	}
	if ctx.Err() != nil {
		return Result{130, strings.Join(append(lines, "Cancelled; Host was not updated."), "\n")}, nil
	}
	var result Result
	if o.Recover {
		result, err = recoverIfPending(ctx, s, exe)
	} else {
		result, err = s.Run(ctx, exe, o)
	}
	appendResult("Host ("+host.Root+")", result, err)
	return Result{code, strings.Join(lines, "\n")}, nil
}

func recoverIfPending(ctx context.Context, s Service, exe string) (Result, error) {
	t, err := s.Files.Select(exe, false)
	if err != nil {
		return Result{}, err
	}
	if s.selectedID != "" && (t.Kind != "store" || t.ID != s.selectedID) {
		return Result{}, fmt.Errorf("registered Store identity changed")
	}
	if err = s.Files.Pending(t); os.IsNotExist(err) {
		return Result{0, "No pending update transaction."}, nil
	} else if err != nil {
		return Result{}, err
	}
	return s.Run(ctx, exe, Options{Recover: true})
}
