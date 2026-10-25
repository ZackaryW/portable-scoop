package resolver

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/metadata"
	"portscoop/internal/registry"
)

type Target struct {
	Identity environment.Identity
	Paths    environment.Paths
	VolumeID string
}
type Discovery struct {
	Files        environment.Filesystem
	Volumes      VolumeSource
	ProbeTimeout time.Duration
}
type candidate struct {
	path, volume string
	priority     int
}

func (d Discovery) files() environment.Filesystem {
	if d.Files == nil {
		return environment.OSFilesystem{}
	}
	return d.Files
}

func candidates(selection Selection, volumes []Volume) []candidate {
	var all []candidate
	seen := map[string]bool{}
	add := func(path, volume string, priority int) {
		key := strings.ToLower(filepath.Clean(path))
		longest := 0
		for _, available := range volumes {
			relative, err := filepath.Rel(available.Root, path)
			if err == nil && (relative == "." || metadata.RelativePath(relative)) && available.ID != "" && len(available.Root) > longest {
				volume = available.ID
				longest = len(available.Root)
			}
		}
		if filepath.IsAbs(path) && !seen[key] {
			seen[key] = true
			all = append(all, candidate{path: path, volume: volume, priority: priority})
		}
	}
	for _, hint := range selection.Hints {
		add(hint.Path, hint.VolumeID, 0)
	}
	for _, hint := range selection.Hints {
		if !metadata.RelativePath(hint.RelativePath) {
			continue
		}
		for _, volume := range volumes {
			priority := 2
			if hint.VolumeID != "" && strings.EqualFold(hint.VolumeID, volume.ID) {
				priority = 1
			}
			add(filepath.Join(volume.Root, hint.RelativePath), volume.ID, priority)
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].priority != all[j].priority {
			return all[i].priority < all[j].priority
		}
		return all[i].path < all[j].path
	})
	return all
}

func (d Discovery) probe(path, id string) (Target, error) {
	identity, paths, err := environment.Load(d.files(), path)
	if err != nil {
		return Target{}, err
	}
	if identity.ID != id {
		return Target{}, fmt.Errorf("candidate identity mismatch")
	}
	executable, err := d.files().Stat(filepath.Join(paths.Root, "portscoop.exe"))
	if err != nil || executable.IsDir() {
		return Target{}, fmt.Errorf("portable launcher is missing")
	}
	return Target{Identity: identity, Paths: paths}, nil
}

// Resolve checks all bounded candidate locations, including those following an
// apparently valid cache hit. It never searches entire drives or invokes Scoop.
func (d Discovery) Resolve(ctx context.Context, selection Selection) (Target, error) {
	if !environment.ValidID(selection.ID) {
		return Target{}, fmt.Errorf("invalid selected identity")
	}
	if selection.Local {
		return d.probe(selection.Paths.Root, selection.ID)
	}
	if d.Volumes == nil {
		return Target{}, fmt.Errorf("central discovery requires an explicit volume source")
	}
	volumes, err := d.Volumes.Volumes(ctx)
	if err != nil {
		return Target{}, err
	}
	paths := candidates(selection, volumes)
	timeout := d.ProbeTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	budget, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	jobs := make(chan candidate, len(paths))
	for _, path := range paths {
		jobs <- path
	}
	close(jobs)
	type result struct {
		target Target
		err    error
	}
	results := make(chan result, len(paths))
	for range min(4, len(paths)) {
		go func() {
			for {
				select {
				case <-budget.Done():
					return
				case path, ok := <-jobs:
					if !ok {
						return
					}
					target, err := d.probe(path.path, selection.ID)
					target.VolumeID = path.volume
					results <- result{target, err}
				}
			}
		}()
	}
	found := map[string]Target{}
	collect := func(item result) {
		if item.err == nil {
			found[strings.ToLower(filepath.Clean(item.target.Paths.Root))] = item.target
		}
	}
	for remaining := len(paths); remaining > 0; remaining-- {
		select {
		case item := <-results:
			collect(item)
		case <-budget.Done():
			if ctx.Err() != nil {
				return Target{}, ctx.Err()
			}
		drain:
			for {
				select {
				case item := <-results:
					collect(item)
				default:
					break drain
				}
			}
			remaining = 0
		}
	}
	if len(found) == 0 {
		return Target{}, fmt.Errorf("environment %s is unavailable; connect its drive or refresh env register with its new path", selection.ID)
	}
	if len(found) > 1 {
		var conflicting []string
		for _, target := range found {
			conflicting = append(conflicting, target.Paths.Root)
		}
		sort.Strings(conflicting)
		return Target{}, fmt.Errorf("ambiguous installation identity %s at %s", selection.ID, strings.Join(conflicting, ", "))
	}
	for _, target := range found {
		return target, nil
	}
	panic("unreachable")
}

// Refresh updates location hints only after a unique, identity-verified target
// has been found. It does not change alias ownership or default selection.
func Refresh(ctx context.Context, store registry.Store, target Target) error {
	if err := target.Identity.Validate(); err != nil {
		return err
	}
	if !filepath.IsAbs(target.Paths.Root) {
		return fmt.Errorf("verified target root must be absolute")
	}
	volume := filepath.VolumeName(target.Paths.Root)
	relative, err := filepath.Rel(volume+string(filepath.Separator), target.Paths.Root)
	if err != nil {
		return err
	}
	return store.Update(ctx, func(document *registry.Document) error {
		for alias, entry := range document.Entries {
			if entry.ID != target.Identity.ID {
				continue
			}
			entry.Path = target.Paths.Root
			entry.RelativePath = relative
			entry.VolumeID = target.VolumeID
			document.Entries[alias] = entry
		}
		return nil
	})
}
