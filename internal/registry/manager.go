package registry

import (
	"context"
	"fmt"
	"path/filepath"

	"portscoop/internal/environment"
	"portscoop/internal/localvol"
)

type Manager struct {
	Store Store
	Files environment.Filesystem
}

func (m Manager) filesystem() environment.Filesystem {
	if m.Files == nil {
		return environment.OSFilesystem{}
	}
	return m.Files
}

func (m Manager) Register(ctx context.Context, path, name string) error {
	alias, err := Alias(name)
	if err != nil {
		return err
	}
	id, paths, err := environment.Load(m.filesystem(), path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(paths.Root)
	if volume == "" || len(volume) != 2 {
		return fmt.Errorf("registration requires a local drive root")
	}
	relative, err := filepath.Rel(volume+string(filepath.Separator), paths.Root)
	if err != nil {
		return err
	}
	entry := Entry{ID: id.ID, Path: paths.Root, RelativePath: relative}
	entry.VolumeID, _ = localvol.Identity(paths.Root)
	return m.Store.Update(ctx, func(d *Document) error {
		if previous, exists := d.Entries[alias]; exists && previous.ID != id.ID {
			return fmt.Errorf("alias %q already belongs to another identity; unregister it first", alias)
		}
		d.Entries[alias] = entry
		return nil
	})
}

func (m Manager) SetDefault(ctx context.Context, name string) error {
	alias, err := Alias(name)
	if err != nil {
		return err
	}
	return m.Store.Update(ctx, func(d *Document) error {
		entry, exists := d.Entries[alias]
		if !exists {
			return fmt.Errorf("environment alias %q is not registered", alias)
		}
		d.DefaultID = entry.ID
		return nil
	})
}

func (m Manager) Unregister(ctx context.Context, name string) error {
	alias, err := Alias(name)
	if err != nil {
		return err
	}
	return m.Store.Update(ctx, func(d *Document) error {
		if _, exists := d.Entries[alias]; !exists {
			return fmt.Errorf("environment alias %q is not registered", alias)
		}
		delete(d.Entries, alias)
		for _, entry := range d.Entries {
			if entry.ID == d.DefaultID {
				return nil
			}
		}
		d.DefaultID = ""
		return nil
	})
}
