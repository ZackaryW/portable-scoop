package scoop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"portscoop/internal/compatibility"
	"portscoop/internal/environment"
	"portscoop/internal/metadata"
)

type FrozenManifest struct {
	Bucket    string                   `json:"bucket"`
	Manifest  Manifest                 `json:"manifest"`
	Decisions []compatibility.Decision `json:"decisions,omitempty"`
	Actions   []compatibility.Action   `json:"actions,omitempty"`
	Downloads []compatibility.Download `json:"downloads,omitempty"`
}

// containedFile resolves reparse points before reading managed metadata.
func containedFile(root, path string) ([]byte, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || !metadata.RelativePath(relative) {
		return nil, fmt.Errorf("managed file escapes portable root: %s", path)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("managed path is not a regular file: %s", path)
	}
	return os.ReadFile(resolved)
}

type manifestSource struct {
	paths         environment.Paths
	buckets       map[string]string
	installedOnly bool
}

func appName(reference string) string {
	parts := strings.Split(strings.ToLower(reference), "/")
	return parts[len(parts)-1]
}

func (s manifestSource) installed(reference string) ([]byte, error) {
	app := appName(reference)
	data, err := containedFile(s.paths.Root, filepath.Join(s.paths.Apps, app, "current", "scoop-manifest.json"))
	if os.IsNotExist(err) {
		return containedFile(s.paths.Root, filepath.Join(s.paths.Apps, app, "current", "manifest.json"))
	}
	return data, err
}

func (s manifestSource) load(reference string) ([]byte, error) {
	if !PackageName(reference) {
		return nil, fmt.Errorf("invalid package reference")
	}
	key := strings.ToLower(reference)
	name := appName(reference)
	if s.installedOnly {
		data, err := s.installed(reference)
		if err == nil {
			return data, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	parts := strings.Split(key, "/")
	var buckets []string
	if len(parts) == 2 {
		buckets = []string{parts[0]}
	} else {
		// Retain the installed bucket rather than silently switching ownership.
		data, err := containedFile(s.paths.Root, filepath.Join(s.paths.Apps, name, "current", "scoop-install.json"))
		if err == nil {
			var info struct {
				Bucket string `json:"bucket"`
			}
			if err := json.Unmarshal(data, &info); err != nil {
				return nil, fmt.Errorf("invalid installed source for %s", name)
			}
			if !PackageName(info.Bucket) || strings.Contains(info.Bucket, "/") {
				return nil, fmt.Errorf("%s has an unsupported installed source", name)
			}
			buckets = []string{strings.ToLower(info.Bucket)}
		} else if !os.IsNotExist(err) {
			return nil, err
		} else {
			entries, err := os.ReadDir(s.paths.Buckets)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if entry.IsDir() && PackageName(entry.Name()) {
					buckets = append(buckets, entry.Name())
				}
			}
			sort.Strings(buckets)
			for i, bucket := range buckets {
				if bucket == "main" {
					buckets = append([]string{bucket}, append(buckets[:i], buckets[i+1:]...)...)
					break
				}
			}
		}
	}
	for _, bucket := range buckets {
		base := filepath.Join(s.paths.Buckets, bucket)
		if info, err := os.Stat(filepath.Join(base, "bucket")); err == nil && info.IsDir() {
			base = filepath.Join(base, "bucket")
		}
		data, err := containedFile(s.paths.Root, filepath.Join(base, name+".json"))
		if err == nil {
			s.buckets[key] = bucket
			return data, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("no root-local manifest found for %s", reference)
}

func installedNames(paths environment.Paths) ([]string, error) {
	entries, err := os.ReadDir(paths.Apps)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() == "scoop" {
			continue
		}
		if !entry.IsDir() || !PackageName(entry.Name()) {
			return nil, fmt.Errorf("unsupported installed app directory %q", entry.Name())
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

var reinstallBackupPattern = regexp.MustCompile(`^_([a-zA-Z0-9][a-zA-Z0-9._+-]*)\.old(?:\([1-9][0-9]*\))?$`)

// Validate every saved version that uninstall/update/reset/cleanup may read.
// Old versions can contain uninstall metadata even when current is safe.
func validateSaved(paths environment.Paths, names []string) error {
	for _, reference := range names {
		name := appName(reference)
		directory := filepath.Join(paths.Apps, name)
		entries, err := os.ReadDir(directory)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name() == "current" {
				continue
			}
			// Forced reinstall preserves the previous physical version using
			// Scoop's _version.old or _version.old(n) format. Validate its saved
			// metadata too; skipping backups would bypass uninstall preflight.
			backup := reinstallBackupPattern.FindStringSubmatch(entry.Name())
			if !entry.IsDir() || (!versionPattern.MatchString(entry.Name()) && backup == nil) || !metadata.RelativePath(entry.Name()) {
				return fmt.Errorf("unsupported installed version for %s", name)
			}
			data, err := containedFile(paths.Root, filepath.Join(directory, entry.Name(), "scoop-manifest.json"))
			if os.IsNotExist(err) {
				data, err = containedFile(paths.Root, filepath.Join(directory, entry.Name(), "manifest.json"))
			}
			if err != nil {
				return fmt.Errorf("saved manifest %s/%s: %w", name, entry.Name(), err)
			}
			manifest, err := DecodeManifest(data)
			if err == nil && backup != nil {
				var original string
				if json.Unmarshal(manifest["version"], &original) != nil || original != backup[1] {
					return fmt.Errorf("reinstall backup version differs from saved manifest for %s", name)
				}
			}
			if err == nil {
				_, err = ValidateManifest(manifest)
			}
			if err != nil {
				return fmt.Errorf("saved manifest %s/%s: %w", name, entry.Name(), err)
			}
			// Prevent installed architecture/source overrides bypassing the selected subset.
			info, err := containedFile(paths.Root, filepath.Join(directory, entry.Name(), "scoop-install.json"))
			if os.IsNotExist(err) {
				info, err = containedFile(paths.Root, filepath.Join(directory, entry.Name(), "install.json"))
			}
			if err != nil {
				return fmt.Errorf("installed metadata %s: %w", name, err)
			}
			var saved struct{ Architecture, Bucket, URL string }
			if err = json.Unmarshal(info, &saved); err != nil || saved.Architecture != "64bit" || saved.URL != "" || !PackageName(saved.Bucket) || strings.Contains(saved.Bucket, "/") {
				return fmt.Errorf("unsupported installed metadata for %s", name)
			}
		}
	}
	return nil
}

// Plan freezes the effective declarations used by the patched engine. It never
// leaves the engine free to reread a changed bucket after Go's preflight.
func Plan(paths environment.Paths, op Operation) (map[string]FrozenManifest, error) {
	result := map[string]FrozenManifest{}
	if !op.Preflight && op.Name != "status" {
		return result, nil
	}
	names := append([]string(nil), op.Packages...)
	if op.Name == "status" || containsWildcard(names) {
		var err error
		names, err = installedNames(paths)
		if err != nil {
			return nil, err
		}
	}
	if err := validateSaved(paths, names); err != nil {
		return nil, err
	}
	source := manifestSource{paths: paths, buckets: map[string]string{}, installedOnly: op.Name == "uninstall" || op.Name == "reset" || op.Name == "cleanup"}
	plan, err := Preflight(names, source.load)
	if err != nil {
		return nil, err
	}
	var closure []string
	for _, item := range plan {
		closure = append(closure, item.Name)
		result[appName(item.Name)] = FrozenManifest{Bucket: source.buckets[strings.ToLower(item.Name)], Manifest: item.Manifest, Decisions: item.Decisions, Actions: item.Actions, Downloads: item.Downloads}
	}
	if err := validateSaved(paths, closure); err != nil {
		return nil, err
	}
	return result, nil
}

func containsWildcard(names []string) bool {
	for _, name := range names {
		if name == "*" {
			return true
		}
	}
	return false
}
