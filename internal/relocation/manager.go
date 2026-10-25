package relocation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"portscoop/internal/environment"
)

type Manager struct{ AfterArtifact func(int) error }

func (m Manager) Capture(paths environment.Paths) error {
	artifacts, err := inventory(paths)
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(paths.State, "prepared.json"), Prepared{SchemaVersion: 1, Root: paths.Root, Artifacts: artifacts})
}

func (m Manager) Prepare(paths environment.Paths) error {
	var state Prepared
	statePath := filepath.Join(paths.State, "prepared.json")
	if err := readJSON(paths.Root, statePath, &state); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		artifacts, scanErr := inventory(paths)
		if scanErr != nil {
			return scanErr
		}
		if len(artifacts) > 0 {
			return fmt.Errorf("managed artifacts have no prepared-root record; restore compatible state before execution")
		}
		return m.Capture(paths)
	}
	if state.SchemaVersion != 1 {
		return fmt.Errorf("unsupported prepared-root schema")
	}
	if _, err := environment.PathsAt(state.Root); err != nil {
		return err
	}
	journalPath := filepath.Join(paths.State, "repair.json")
	var journal Journal
	err := readJSON(paths.Root, journalPath, &journal)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	active := err == nil
	if active {
		if journal.SchemaVersion != 1 || !strings.EqualFold(journal.To, paths.Root) {
			return fmt.Errorf("interrupted repair belongs to another root; return this directory to %s before resuming", journal.To)
		}
		if _, err := environment.PathsAt(journal.From); err != nil {
			return err
		}
	} else {
		journal = Journal{SchemaVersion: 1, From: state.Root, To: paths.Root, Artifacts: state.Artifacts}
		if !strings.EqualFold(state.Root, paths.Root) {
			if err := writeJSON(journalPath, journal); err != nil {
				return err
			}
			active = true
		}
	}
	seen := map[string]bool{}
	for index, artifact := range journal.Artifacts {
		key := strings.ToLower(filepath.ToSlash(artifact.Path))
		if seen[key] {
			return fmt.Errorf("duplicate owned artifact %s", artifact.Path)
		}
		seen[key] = true
		if err := m.apply(paths, journal.From, artifact); err != nil {
			return fmt.Errorf("repair %s: %w", artifact.Path, err)
		}
		if m.AfterArtifact != nil {
			if err := m.AfterArtifact(index); err != nil {
				return err
			}
		}
	}
	// Reject newly introduced/unknown managed files rather than adopting them
	// while preparing an existing environment.
	current, err := inventory(paths)
	if err != nil {
		return err
	}
	if len(current) != len(journal.Artifacts) {
		return fmt.Errorf("managed artifact inventory changed outside a recorded operation; inspect shims and links")
	}
	for _, artifact := range current {
		if !seen[strings.ToLower(artifact.Path)] {
			return fmt.Errorf("untracked managed artifact %s", artifact.Path)
		}
	}
	if active {
		if err := writeJSON(statePath, Prepared{SchemaVersion: 1, Root: paths.Root, Artifacts: current}); err != nil {
			return err
		}
		if err := os.Remove(journalPath); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) apply(paths environment.Paths, from string, a Artifact) error {
	path, err := artifactPath(paths, a)
	if err != nil {
		return err
	}
	if a.Kind == "junction" || a.Kind == "hardlink" {
		target := filepath.Join(paths.Root, filepath.FromSlash(a.Target))
		if err := containedParent(paths.Root, target); err != nil {
			return err
		}
		canonical, err := filepath.EvalSymlinks(target)
		if err != nil || !within(paths.Root, canonical) {
			return fmt.Errorf("link destination escapes root or is unavailable")
		}
		if a.Kind == "junction" {
			actual, err := os.Readlink(path)
			if err != nil {
				if !strings.EqualFold(from, paths.Root) {
					if os.IsNotExist(err) {
						return createJunction(path, target)
					}
					if entries, listErr := os.ReadDir(path); listErr == nil && len(entries) == 0 {
						if err := os.Remove(path); err != nil {
							return err
						}
						return createJunction(path, target)
					}
				}
				return fmt.Errorf("owned junction unavailable: %w", err)
			}
			if !filepath.IsAbs(actual) {
				actual = filepath.Join(filepath.Dir(path), actual)
			}
			previous := filepath.Join(from, filepath.FromSlash(a.Target))
			if !strings.EqualFold(filepath.Clean(actual), previous) && !strings.EqualFold(filepath.Clean(actual), target) {
				return fmt.Errorf("link points outside its recorded destination: %s", actual)
			}
			if strings.EqualFold(filepath.Clean(actual), target) {
				return nil
			}
			if err := removeLink(path); err != nil {
				return err
			}
			return createJunction(path, target)
		}
		left, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) && !strings.EqualFold(from, paths.Root) {
				return os.Link(target, path)
			}
			return err
		}
		right, err := os.Stat(target)
		if err != nil {
			return err
		}
		if !left.Mode().IsRegular() || !right.Mode().IsRegular() {
			return fmt.Errorf("unsupported hardlink type")
		}
		if os.SameFile(left, right) {
			return nil
		}
		// Cross-volume copies can split hardlinks. Rejoin only identical copies.
		before, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		after, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if !bytes.Equal(before, after) {
			return fmt.Errorf("persisted hardlink copies differ; reconcile data before retrying")
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		return os.Link(target, path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("owned file is a reparse point")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if a.Kind == "binary" {
		if digest(data) != a.Digest {
			return fmt.Errorf("binary shim changed")
		}
		return validateBinaryShim(paths, data)
	}
	desired := a.Content
	if a.Kind == "shim" {
		if err := validateShim(a.Content, from); err != nil {
			return err
		}
		desired = []byte(relocateText(string(a.Content), from, paths.Root))
	} else if a.Kind == "environment" {
		updated, err := environmentContent(a.Content, from, paths.Root)
		if err != nil {
			return err
		}
		if !strings.EqualFold(from, paths.Root) {
			desired = updated
		}
	}
	if !bytes.Equal(data, a.Content) && !bytes.Equal(data, desired) {
		return fmt.Errorf("owned file changed outside the recorded operation")
	}
	if bytes.Equal(data, desired) {
		return nil
	}
	return replaceFile(path, desired)
}
