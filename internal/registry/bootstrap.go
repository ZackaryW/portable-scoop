package registry

import (
	"context"
	"errors"
	"fmt"
	"os"

	"portscoop/internal/environment"
)

var errDefaultConfigured = errors.New("a default Store is already configured")

// Bootstrap holds the registry lock across the first Store's initialization and
// publication. Concurrent Host setup cannot replace a default selected by another
// setup. A ready Store is retained if registry publication subsequently fails.
func (m Manager) Bootstrap(ctx context.Context, root string, initialize func(context.Context, string, string) (environment.Identity, error)) (bool, error) {
	if initialize == nil {
		return false, fmt.Errorf("default Store initialization is unavailable")
	}
	initialized := false
	err := m.Store.Update(ctx, func(d *Document) error {
		if d.DefaultID != "" {
			var inspectionErr error
			for _, entry := range d.Entries {
				if entry.ID != d.DefaultID {
					continue
				}
				id, _, err := environment.Load(m.filesystem(), entry.Path)
				if err == nil && id.ID == d.DefaultID {
					return errDefaultConfigured
				}
				if err == nil {
					inspectionErr = fmt.Errorf("default Store identity does not match its registration at %s", entry.Path)
				} else if !errors.Is(err, os.ErrNotExist) {
					inspectionErr = err
				}
			}
			if inspectionErr != nil {
				return fmt.Errorf("cannot inspect the existing default Store; setup did not create or select a new Store: %w", inspectionErr)
			}
		}
		alias := "default"
		if previous, exists := d.Entries[alias]; exists {
			if d.DefaultID == "" || previous.ID != d.DefaultID {
				return fmt.Errorf("alias default is already registered; choose its default explicitly or unregister it first")
			}
			// The requested bootstrap replaces a missing default selection, not
			// its registration. Keep its alias available for later reconnection.
			for suffix := 2; ; suffix++ {
				alias = fmt.Sprintf("default-%d", suffix)
				if _, exists := d.Entries[alias]; !exists {
					break
				}
			}
		}
		id, err := initialize(ctx, root, "Default")
		if err != nil {
			return err
		}
		initialized = true
		loaded, paths, err := environment.Load(m.filesystem(), root)
		if err != nil {
			return err
		}
		if loaded != id {
			return fmt.Errorf("initialized Store identity changed before registration")
		}
		entry, err := registrationEntry(id, paths)
		if err != nil {
			return err
		}
		d.Entries[alias] = entry
		d.DefaultID = id.ID
		return nil
	})
	if errors.Is(err, errDefaultConfigured) {
		return false, nil
	}
	if err != nil && initialized {
		return true, fmt.Errorf("Store initialized at %s, but default registration failed; preserve it and use env register/default to finish: %w", root, err)
	}
	return initialized, err
}
