package environment

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Filesystem permits deterministic identity/discovery tests without scanning or
// reading the developer's actual drives or Scoop configuration.
type Filesystem interface {
	ReadFile(string) ([]byte, error)
	Stat(string) (fs.FileInfo, error)
	Canonical(string) (string, error)
}

type OSFilesystem struct{}

func (OSFilesystem) ReadFile(path string) ([]byte, error)  { return os.ReadFile(path) }
func (OSFilesystem) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }
func (OSFilesystem) Canonical(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

// Load reads only the fixed marker relative to the canonical root. It never
// searches PATH or falls back to another installation.
func Load(files Filesystem, root string) (Identity, Paths, error) {
	canonical, err := files.Canonical(root)
	if err != nil {
		return Identity{}, Paths{}, fmt.Errorf("resolve environment root: %w", err)
	}
	paths, err := PathsAt(canonical)
	if err != nil {
		return Identity{}, Paths{}, err
	}
	data, err := files.ReadFile(filepath.Join(paths.Root, filepath.FromSlash(MarkerRelative)))
	if err != nil {
		return Identity{}, Paths{}, fmt.Errorf("read environment identity: %w", err)
	}
	id, err := DecodeIdentity(data)
	return id, paths, err
}
