package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"portscoop/internal/atomicfile"
	"portscoop/internal/filelock"
)

// Store is explicit and injectable: tests never select a host registry by
// accident. Replace permits failure injection immediately before publication.
type Store struct {
	Path    string
	Replace func(string, string) error
}

func (s Store) Read() (Document, error) {
	if !filepath.IsAbs(s.Path) {
		return Document{}, fmt.Errorf("registry path must be absolute")
	}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return Document{}, err
	}
	return Decode(data)
}

func (s Store) Update(ctx context.Context, edit func(*Document) error) error {
	if !filepath.IsAbs(s.Path) {
		return fmt.Errorf("registry path must be absolute")
	}
	parent := filepath.Dir(s.Path)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	lock, err := filelock.Acquire(ctx, s.Path+".lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	document, err := s.Read()
	if err != nil {
		return err
	}
	if err := edit(&document); err != nil {
		return err
	}
	if err := document.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(parent, ".registry-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	replace := s.Replace
	if replace == nil {
		replace = atomicfile.Replace
	}
	return replace(file.Name(), s.Path)
}
