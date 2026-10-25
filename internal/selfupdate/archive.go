package selfupdate

import (
	"os"
	"path/filepath"

	"portscoop/internal/runtimeinstall"
)

// VerifyArchive checks a completed release ZIP using the same extraction and
// distribution validation as update staging. It only writes owned temporary
// files; it neither selects an installation nor runs candidate executables.
func VerifyArchive(path string) error {
	root, err := os.MkdirTemp("", "portscoop-archive-check-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if err = extractArchive(path, root); err != nil {
		return err
	}
	_, _, err = runtimeinstall.VerifyDistribution(filepath.Join(root, "candidate"), false)
	return err
}
