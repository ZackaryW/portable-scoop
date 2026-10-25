package selfupdate

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"portscoop/internal/environment"
	"portscoop/internal/runtimeinstall"
)

const backendMetadata = ".portscoop/backend/backend.json"

// backendSource is a closed projection, never a manifest-provided destination.
func backendSource(p string) (string, bool) {
	if p == backendMetadata {
		return "backend/backend.json", true
	}
	for _, pair := range [][2]string{{".portscoop/backend/", "backend/adapter/"}, {"scoop/apps/scoop/current/", "backend/engine/"}} {
		if strings.HasPrefix(p, pair[0]) && len(p) > len(pair[0]) {
			return pair[1] + strings.TrimPrefix(p, pair[0]), true
		}
	}
	return "", false
}

func backendInventory(root, metadata string, candidate bool, release runtimeinstall.Release) (map[string]string, error) {
	var b environment.BackendMetadata
	if err := runtimeinstall.Read(root, metadata, &b); err != nil {
		return nil, err
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	result := map[string]string{}
	for p, digest := range b.Artifacts {
		source, ok := backendSource(p)
		if !ok || p == backendMetadata || p != filepath.ToSlash(filepath.Clean(p)) {
			return nil, fmt.Errorf("artifact outside managed Store backend: %s", p)
		}
		if _, err := runtimeinstall.CheckPath(root, p); err != nil {
			return nil, err
		}
		if candidate {
			if release.Artifacts[source] != digest {
				return nil, fmt.Errorf("backend artifact differs from release inventory: %s", p)
			}
		} else {
			source = p
		}
		h, err := runtimeinstall.Hash(root, source)
		if err != nil || h != digest {
			return nil, fmt.Errorf("backend ownership drift: %s", p)
		}
		result[p] = digest
	}
	h, err := runtimeinstall.Hash(root, metadata)
	if err != nil {
		return nil, err
	}
	result[backendMetadata] = h
	// Every executable backend file must be owned. Preserving an unlisted
	// library would produce a Store that its runtime correctly refuses to use.
	for _, namespace := range []string{".portscoop/backend/", "scoop/apps/scoop/current/"} {
		directory := strings.TrimSuffix(namespace, "/")
		if candidate {
			source, _ := backendSource(namespace + "placeholder")
			directory = filepath.ToSlash(filepath.Dir(source))
		}
		path, err := runtimeinstall.CheckPath(root, directory)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(path, func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			owned := rel
			if candidate {
				owned = namespace + strings.TrimPrefix(rel, directory+"/")
			}
			if result[owned] == "" {
				return fmt.Errorf("unowned managed backend file: %s", rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func addInventory(to, from map[string]string) {
	for p, h := range from {
		to[p] = h
	}
}

// Snapshot the verified installed controller, not the downloaded candidate.
// Recovery remains valid after that Host has upgraded or disconnected.
func prepareController(tx *Transaction) error {
	c := tx.Controller
	if tx.Target.Kind != "store" || c.Kind != "host" || !environment.ValidID(c.ID) {
		return fmt.Errorf("invalid Store update controller")
	}
	r, d, err := runtimeinstall.VerifyDistribution(c.Root, true)
	if err != nil {
		return err
	}
	if d.Version != c.Version {
		return fmt.Errorf("controller changed during preparation")
	}
	for p := range r.Artifacts {
		if err = copyFile(c.Root, p, tx.Directory, "controller/"+p); err != nil {
			return err
		}
	}
	if err = copyFile(c.Root, "release.json", tx.Directory, "controller/release.json"); err != nil {
		return err
	}
	return copyFile(tx.Directory, "controller/psp.exe", tx.Directory, "worker.exe")
}

func verifyController(tx *Transaction) error {
	c := tx.Controller
	if tx.Target.Kind != "store" || c.Kind != "host" || !environment.ValidID(c.ID) || !filepath.IsAbs(c.Root) {
		return fmt.Errorf("invalid Store update controller")
	}
	r, d, err := runtimeinstall.VerifyDistribution(filepath.Join(tx.Directory, "controller"), false)
	if err != nil {
		return err
	}
	if d.Version != c.Version || r.Artifacts["psp.exe"] != tx.WorkerHash {
		return fmt.Errorf("worker is not the verified controlling Host runtime")
	}
	return nil
}
