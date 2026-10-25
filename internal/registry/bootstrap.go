package registry

import (
	"context"
	"errors"
	"fmt"

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
			return errDefaultConfigured
		}
		if _, exists := d.Entries["default"]; exists {
			return fmt.Errorf("alias default is already registered; choose its default explicitly or unregister it first")
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
		d.Entries["default"] = entry
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
