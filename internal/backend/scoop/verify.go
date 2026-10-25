package scoop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"portscoop/internal/environment"
)

const UpstreamRevision = "e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f"
const PatchRevision = "portable-zip-v1.1"
const MetadataRelative = ".portscoop/backend/backend.json"

// Verify covers every shipped engine/adapter file, including additions. Scoop
// can dot-source libraries, so verifying just bin/scoop.ps1 is insufficient.
func Verify(paths environment.Paths) (environment.BackendMetadata, error) {
	var backend environment.BackendMetadata
	expected, err := environment.PathsAt(paths.Root)
	if err != nil || expected != paths {
		return backend, fmt.Errorf("invalid backend path context")
	}
	data, err := containedFile(paths.Root, filepath.Join(paths.Root, MetadataRelative))
	if err == nil {
		backend, err = environment.DecodeBackend(data)
	}
	if err != nil {
		return backend, fmt.Errorf("managed backend unavailable; restore a compatible release: %w", err)
	}
	if backend.UpstreamRevision != UpstreamRevision || backend.PatchRevision != PatchRevision {
		return backend, fmt.Errorf("incompatible managed backend; restore a compatible release")
	}
	artifacts := map[string]string{}
	for relative, digest := range backend.Artifacts {
		key := strings.ToLower(filepath.ToSlash(filepath.Clean(relative)))
		if !strings.HasPrefix(key, "scoop/apps/scoop/current/") && key != ".portscoop/backend/entry.ps1" && key != ".portscoop/backend/request-reader.ps1" {
			return backend, fmt.Errorf("unexpected backend artifact %s", relative)
		}
		artifacts[key] = digest
		data, err := containedFile(paths.Root, filepath.Join(paths.Root, relative))
		if err != nil {
			return backend, fmt.Errorf("managed backend artifact %s: %w", relative, err)
		}
		hash := sha256.Sum256(data)
		if !strings.EqualFold(digest, hex.EncodeToString(hash[:])) {
			return backend, fmt.Errorf("managed backend artifact changed: %s; restore a compatible release", relative)
		}
	}
	for _, required := range []string{environment.EngineEntryRelative, environment.AdapterRelative, ".portscoop/backend/request-reader.ps1", "scoop/apps/scoop/current/lib/system.ps1", "scoop/apps/scoop/current/lib/manifest.ps1"} {
		if artifacts[required] == "" {
			return backend, fmt.Errorf("backend does not verify required artifact %s", required)
		}
	}
	for _, directory := range []string{filepath.Dir(filepath.Dir(paths.Engine)), filepath.Dir(paths.Adapter)} {
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(paths.Root, path)
			if err != nil {
				return err
			}
			key := strings.ToLower(filepath.ToSlash(relative))
			if key == MetadataRelative {
				return nil
			}
			if artifacts[key] == "" {
				return fmt.Errorf("unverified managed backend file: %s", relative)
			}
			return nil
		})
		if err != nil {
			return backend, err
		}
	}
	config, err := containedFile(paths.Root, paths.Config)
	if err != nil {
		return backend, fmt.Errorf("portable config unavailable: %w", err)
	}
	var object map[string]json.RawMessage
	if err = json.Unmarshal(config, &object); err != nil || object == nil {
		return backend, fmt.Errorf("invalid portable config")
	}
	return backend, nil
}
