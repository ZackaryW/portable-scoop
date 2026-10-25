package resolver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
)

type Selection struct {
	ID       string
	Local    bool
	Identity environment.Identity
	Paths    environment.Paths
	Hints    []registry.Entry
}

type RegistryReader func() (registry.Document, error)

// Select never inspects the working directory. A direct portable invocation
// without a selector does not need a readable (or even existing) host registry.
func Select(files environment.Filesystem, executable string, read RegistryReader, selector string) (Selection, error) {
	if files == nil {
		files = environment.OSFilesystem{}
	}
	if !filepath.IsAbs(executable) {
		return Selection{}, fmt.Errorf("launcher path must be absolute")
	}
	root := filepath.Dir(executable)
	control := filepath.Join(root, ".portscoop")
	_, controlErr := files.Stat(control)
	local := controlErr == nil
	if controlErr != nil && !errors.Is(controlErr, os.ErrNotExist) {
		return Selection{}, fmt.Errorf("inspect adjacent portable state: %w", controlErr)
	}
	var identity environment.Identity
	var paths environment.Paths
	var err error
	if local {
		identity, paths, err = environment.Load(files, root)
		if err != nil {
			return Selection{}, fmt.Errorf("invalid adjacent portable environment: %w", err)
		}
		if selector == "" {
			return Selection{ID: identity.ID, Local: true, Identity: identity, Paths: paths}, nil
		}
	}
	if read == nil {
		return Selection{}, fmt.Errorf("central selection requires a host registry")
	}
	document, err := read()
	if err != nil {
		return Selection{}, err
	}
	if err := document.Validate(); err != nil {
		return Selection{}, err
	}
	id := document.DefaultID
	if selector != "" {
		alias, err := registry.Alias(selector)
		if err != nil {
			return Selection{}, err
		}
		entry, exists := document.Entries[alias]
		if !exists {
			return Selection{}, fmt.Errorf("environment alias %q is not registered", alias)
		}
		id = entry.ID
	}
	if id == "" {
		return Selection{}, fmt.Errorf("select an environment with --env or configure env default")
	}
	if local {
		if id != identity.ID {
			return Selection{}, fmt.Errorf("portable launcher cannot target a different environment identity")
		}
		return Selection{ID: id, Local: true, Identity: identity, Paths: paths}, nil
	}
	var hints []registry.Entry
	for _, entry := range document.Entries {
		if entry.ID == id {
			hints = append(hints, entry)
		}
	}
	sort.Slice(hints, func(i, j int) bool { return hints[i].Path < hints[j].Path })
	return Selection{ID: id, Hints: hints}, nil
}
