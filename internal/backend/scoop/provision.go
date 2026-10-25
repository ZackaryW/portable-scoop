package scoop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"portscoop/internal/atomicfile"
	"portscoop/internal/environment"
	"portscoop/internal/filelock"
	"portscoop/internal/metadata"
	"portscoop/internal/relocation"
	"portscoop/internal/runner"
	"portscoop/internal/runtimeinstall"
)

type Release struct {
	SchemaVersion int               `json:"schema_version"`
	PatchRevision string            `json:"patch_revision"`
	Artifacts     map[string]string `json:"artifacts"`
}

func VerifyRelease(root string) (Release, error) {
	return verifyRelease(root, false)
}

// An installed Host may own one ready default Store outside its release
// inventory. Downloaded payload verification remains strict.
func verifyRelease(root string, allowDefaultStore bool) (Release, error) {
	// The updater owns installed controls separately from the strict downloaded
	// distribution. Both paths still validate every inventoried source file.
	if _, err := os.Stat(filepath.Join(root, runtimeinstall.DescriptorName)); err == nil {
		r, _, err := runtimeinstall.VerifyDistribution(root, allowDefaultStore)
		if err == nil && r.PatchRevision != PatchRevision {
			err = fmt.Errorf("incompatible release payload")
		}
		return Release{r.SchemaVersion, r.PatchRevision, r.Artifacts}, err
	}
	var release Release
	root, err := (environment.OSFilesystem{}).Canonical(root)
	if err != nil {
		return release, fmt.Errorf("resolve release root: %w", err)
	}
	data, err := containedFile(root, filepath.Join(root, "release.json"))
	if err == nil {
		err = metadata.Decode(data, &release)
	}
	if err != nil {
		return release, fmt.Errorf("verified Portscoop release payload required: %w", err)
	}
	if release.SchemaVersion != 1 || release.PatchRevision != PatchRevision {
		return release, fmt.Errorf("incompatible release payload")
	}
	for _, required := range []string{"portscoop.exe", "psp.exe", "pspe.exe", "backend/backend.json", "backend/adapter/entry.ps1", "backend/engine/bin/scoop.ps1"} {
		if release.Artifacts[required] == "" {
			return release, fmt.Errorf("release missing %s", required)
		}
	}
	if release.Artifacts["portscoop.exe"] != release.Artifacts["psp.exe"] {
		return release, fmt.Errorf("release launchers must be identical")
	}
	seen := map[string]bool{}
	for relative, digest := range release.Artifacts {
		key := strings.ToLower(filepath.ToSlash(filepath.Clean(relative)))
		if !metadata.RelativePath(relative) || seen[key] || !hashPattern.MatchString(digest) {
			return release, fmt.Errorf("invalid release artifact %s", relative)
		}
		seen[key] = true
		data, err := containedFile(root, filepath.Join(root, relative))
		if err != nil {
			return release, err
		}
		hash := sha256.Sum256(data)
		if !strings.EqualFold(digest, hex.EncodeToString(hash[:])) {
			return release, fmt.Errorf("release artifact changed: %s", relative)
		}
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		key := strings.ToLower(filepath.ToSlash(relative))
		if allowDefaultStore && key == environment.DefaultStoreDirectory {
			if !entry.IsDir() || entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return fmt.Errorf("Host default Store must be a real directory")
			}
			if _, _, err := environment.Load(environment.OSFilesystem{}, path); err != nil {
				return fmt.Errorf("Host default Store is not ready: %w", err)
			}
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if key != "release.json" && key != ".portscoop-init.lock" && !seen[key] {
			return fmt.Errorf("unrelated file in release directory: %s", relative)
		}
		return nil
	})
	return release, err
}

type Initializer struct {
	ReleaseRoot   string
	Runner        runner.ProcessRunner
	BootstrapMain func(context.Context, environment.Paths) error // test acquisition boundary; nil uses pinned managed Git
	BeforePublish func() error                                   // deterministic provisioning fault injection
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("release payload contains a reparse point: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
}

func (i Initializer) Init(ctx context.Context, root, name string) (environment.Identity, error) {
	id, err := environment.NewIdentity(name)
	if err != nil {
		return id, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return id, err
	}
	if _, err = environment.PathsAt(root); err != nil {
		return id, err
	}
	releaseRoot, err := filepath.Abs(i.ReleaseRoot)
	if err != nil {
		return id, err
	}
	releaseRoot, err = filepath.EvalSymlinks(releaseRoot)
	if err != nil {
		return id, err
	}
	release, err := verifyRelease(releaseRoot, true)
	if err != nil {
		return id, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return id, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return id, err
	}
	lock, err := filelock.Acquire(ctx, filepath.Join(root, ".portscoop-init.lock"))
	if err != nil {
		return id, err
	}
	defer lock.Close()
	entries, err := os.ReadDir(root)
	if err != nil {
		return id, err
	}
	preexisting := false
	for _, entry := range entries {
		if entry.Name() != ".portscoop-init.lock" {
			preexisting = true
		}
	}
	if preexisting {
		existing, err := VerifyRelease(root)
		if err != nil || existing.Artifacts["portscoop.exe"] != release.Artifacts["portscoop.exe"] {
			return id, fmt.Errorf("destination must be empty or contain only this verified release payload; existing files were preserved")
		}
	}
	stage, err := os.MkdirTemp(root, ".portscoop-init-")
	if err != nil {
		return id, err
	}
	// stage was created directly under canonical root by this invocation.
	defer os.RemoveAll(stage)
	paths, _ := environment.PathsAt(stage)
	for _, directory := range []string{paths.State, filepath.Dir(paths.Adapter), filepath.Dir(filepath.Dir(paths.Engine)), paths.Buckets, paths.Cache, paths.Persist, paths.Shims, paths.Global, paths.Temp} {
		if err = os.MkdirAll(directory, 0700); err != nil {
			return id, err
		}
	}
	if err = copyTree(filepath.Join(releaseRoot, "backend", "engine"), filepath.Dir(filepath.Dir(paths.Engine))); err != nil {
		return id, err
	}
	if err = copyTree(filepath.Join(releaseRoot, "backend", "adapter"), filepath.Dir(paths.Adapter)); err != nil {
		return id, err
	}
	if err = copyTree(filepath.Join(releaseRoot, "backend", "tools"), filepath.Join(stage, ToolsRelative)); err != nil {
		return id, err
	}
	for _, pair := range [][2]string{{filepath.Join(releaseRoot, "backend", "backend.json"), filepath.Join(stage, MetadataRelative)}, {filepath.Join(releaseRoot, "portscoop.exe"), filepath.Join(stage, "portscoop.exe")}, {filepath.Join(releaseRoot, "psp.exe"), filepath.Join(stage, "psp.exe")}, {filepath.Join(releaseRoot, "pspe.exe"), filepath.Join(stage, "pspe.exe")}} {
		data, err := os.ReadFile(pair[0])
		if err != nil {
			return id, err
		}
		if err = os.WriteFile(pair[1], data, 0600); err != nil {
			return id, err
		}
	}
	if err = verifyLaunchers(stage, release); err != nil {
		return id, err
	}
	var descriptor runtimeinstall.Descriptor
	if err = runtimeinstall.Read(releaseRoot, runtimeinstall.DescriptorName, &descriptor); err == nil {
		if err = descriptor.Validate(); err != nil {
			return id, err
		}
		receipt := runtimeinstall.NewReceipt(id.ID, descriptor, runtimeinstall.Release{SchemaVersion: release.SchemaVersion, PatchRevision: release.PatchRevision, Artifacts: release.Artifacts})
		if err = runtimeinstall.Write(stage, runtimeinstall.ReceiptName, receipt); err != nil {
			return id, err
		}
	} else if !os.IsNotExist(err) {
		return id, err
	}
	if err = os.WriteFile(paths.Config, []byte("{}\n"), 0600); err != nil {
		return id, err
	}
	if _, err = Verify(paths); err != nil {
		return id, err
	}
	if err = CheckRuntime(ctx, paths, i.Runner); err != nil {
		return id, err
	}
	bootstrap := i.BootstrapMain
	if bootstrap == nil {
		bootstrap = (Bootstrapper{}).ensureMain
	}
	if err = bootstrap(ctx, paths); err != nil {
		return id, fmt.Errorf("Store bootstrap: %w", err)
	}
	if i.BeforePublish != nil {
		if err = i.BeforePublish(); err != nil {
			return id, err
		}
	}
	// No ready marker is present during publication. A media failure here leaves
	// diagnostic partial state, which is never adopted as a ready environment.
	for _, directory := range []string{"scoop", ".portscoop"} {
		if err = os.Rename(filepath.Join(stage, directory), filepath.Join(root, directory)); err != nil {
			return id, fmt.Errorf("initialization publication incomplete at %s; preserve files and inspect before retrying: %w", root, err)
		}
	}
	if !preexisting {
		for _, launcher := range []string{"portscoop.exe", "psp.exe", "pspe.exe"} {
			if err = os.Rename(filepath.Join(stage, launcher), filepath.Join(root, launcher)); err != nil {
				return id, err
			}
		}
	}
	if err = verifyLaunchers(root, release); err != nil {
		return id, err
	}
	finalPaths, _ := environment.PathsAt(root)
	if err = (relocation.Manager{}).Capture(finalPaths); err != nil {
		return id, err
	}
	data, _ := json.MarshalIndent(id, "", "  ")
	marker := filepath.Join(root, environment.MarkerRelative)
	file, err := os.CreateTemp(filepath.Dir(marker), "ready-*.tmp")
	if err != nil {
		return id, err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = atomicfile.Replace(file.Name(), marker)
	}
	if err != nil {
		return id, err
	}
	return id, nil
}

func verifyLaunchers(root string, release Release) error {
	for _, name := range []string{"portscoop.exe", "psp.exe", "pspe.exe"} {
		data, err := containedFile(root, filepath.Join(root, name))
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(hash[:]), release.Artifacts[name]) {
			return fmt.Errorf("copied release launcher changed: %s", name)
		}
	}
	return nil
}
