package runtimeinstall

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"portscoop/internal/environment"
)

// DistributionPath is the fixed namespace that Host updates may own.
func DistributionPath(p string) bool {
	p = strings.ToLower(p)
	for _, n := range []string{"portscoop.exe", "psp.exe", "pspe.exe", "release.json", DescriptorName, "readme.md", "license", "setup.ps1", "get.ps1", "scripts/install.ps1"} {
		if p == n {
			return true
		}
	}
	return strings.HasPrefix(p, "backend/") || strings.HasPrefix(p, "docs/")
}
func VerifyDistribution(root string, installed bool) (Release, Descriptor, error) {
	var r Release
	var d Descriptor
	if err := Read(root, "release.json", &r); err != nil {
		return r, d, fmt.Errorf("complete release required: %w", err)
	}
	if r.SchemaVersion != 1 || r.PatchRevision == "" || len(r.Artifacts) == 0 {
		return r, d, fmt.Errorf("unsupported release inventory")
	}
	seen := map[string]bool{}
	for p, digest := range r.Artifacts {
		key := strings.ToLower(p)
		if p != filepath.ToSlash(filepath.Clean(p)) || !DistributionPath(p) || p == "release.json" || seen[key] || !DigestPattern.MatchString(digest) {
			return r, d, fmt.Errorf("unsupported distribution artifact %s", p)
		}
		seen[key] = true
		h, err := Hash(root, p)
		if err != nil {
			return r, d, err
		}
		if h != digest {
			return r, d, fmt.Errorf("distribution artifact changed: %s", p)
		}
	}
	for _, n := range append(append([]string{}, Launchers...), DescriptorName, "backend/backend.json", "backend/adapter/entry.ps1", "backend/engine/bin/scoop.ps1") {
		if r.Artifacts[n] == "" {
			return r, d, fmt.Errorf("missing distribution artifact %s", n)
		}
	}
	if r.Artifacts["portscoop.exe"] != r.Artifacts["psp.exe"] {
		return r, d, fmt.Errorf("management launchers differ")
	}
	if err := Read(root, DescriptorName, &d); err != nil {
		return r, d, err
	}
	if err := d.Validate(); err != nil {
		return r, d, err
	}
	var backend environment.BackendMetadata
	if err := Read(root, "backend/backend.json", &backend); err != nil {
		return r, d, err
	}
	if err := backend.Validate(); err != nil {
		return r, d, err
	}
	if backend.PatchRevision != r.PatchRevision {
		return r, d, fmt.Errorf("backend and release revisions differ")
	}
	matched := false
	for _, c := range d.Backends {
		if c == (Contract{backend.SchemaVersion, backend.ProtocolMajor, backend.UpstreamRevision, backend.PatchRevision}) {
			matched = true
		}
	}
	if !matched {
		return r, d, fmt.Errorf("descriptor does not support its shipped backend")
	}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		key := strings.ToLower(rel)
		if installed && key == "store" {
			if _, err := CheckPath(root, rel); err != nil {
				return err
			}
			if !e.IsDir() {
				return fmt.Errorf("invalid default Store")
			}
			if _, _, err := environment.Load(environment.OSFilesystem{}, path); err != nil {
				return err
			}
			return filepath.SkipDir
		}
		if installed && key == HostControl {
			var h HostIdentity
			if err := Read(root, HostControl+"/identity.json", &h); err != nil {
				return err
			}
			if h.Schema != 1 || !environment.ValidID(h.ID) {
				return fmt.Errorf("unowned Host controls")
			}
			return filepath.SkipDir
		}
		if installed && !DistributionPath(key) && key != "backend" && key != "docs" && key != "scripts" {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if _, err := CheckPath(root, rel); err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		if key != "release.json" && key != ".portscoop-init.lock" && !seen[key] {
			return fmt.Errorf("unowned distribution file %s", rel)
		}
		return nil
	})
	return r, d, err
}

func EnsureHost(t Target) (Target, error) {
	if t.Kind != "host" {
		return t, nil
	}
	control := filepath.Join(t.Root, HostControl)
	if _, err := os.Lstat(control); err == nil {
		var id HostIdentity
		if err = Read(t.Root, HostControl+"/identity.json", &id); err != nil {
			return t, err
		}
		if id.Schema != 1 || !environment.ValidID(id.ID) {
			return t, fmt.Errorf("unowned Host controls")
		}
		t.ID = id.ID
		return t, nil
	} else if !os.IsNotExist(err) {
		return t, err
	}
	id, err := environment.NewIdentity("Host")
	if err != nil {
		return t, err
	}
	// Mkdir (not MkdirAll) refuses a concurrent or unrelated existing control.
	if err = os.Mkdir(control, 0700); err != nil {
		return t, err
	}
	if err = Write(t.Root, HostControl+"/identity.json", HostIdentity{1, id.ID}); err != nil {
		return t, err
	}
	t.ID = id.ID
	return t, nil
}
