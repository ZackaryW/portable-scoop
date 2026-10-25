package relocation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/filelock"
)

type Coordinator struct {
	BusyTimeout time.Duration
	Prepare     func(environment.Paths) error
	Capture     func(environment.Paths) error
	Diagnostic  io.Writer
	Operation   string
}

// Do serializes preparation, execution, and artifact capture for every backend
// operation, including reads whose upstream implementation can update caches.
func (c Coordinator) Do(ctx context.Context, local environment.Context, execute func() (int, error)) (int, error) {
	id, paths, err := environment.Load(environment.OSFilesystem{}, local.Paths.Root)
	if err != nil {
		return 0, err
	}
	if id.ID != local.Identity.ID {
		return 0, fmt.Errorf("environment identity changed before operation locking")
	}
	if err := validateLayout(paths); err != nil {
		return 0, err
	}
	if err = containedParent(paths.Root, filepath.Join(paths.State, "operation.lock")); err != nil {
		return 0, err
	}
	if err = os.MkdirAll(paths.State, 0700); err != nil {
		return 0, err
	}
	if err = containedParent(paths.Root, filepath.Join(paths.State, "operation.lock")); err != nil {
		return 0, err
	}
	budget := c.BusyTimeout
	if budget <= 0 {
		budget = 5 * time.Second
	}
	lockContext, cancel := context.WithTimeout(ctx, budget)
	lock, err := filelock.Acquire(lockContext, filepath.Join(paths.State, "operation.lock"))
	cancel()
	if err != nil {
		return 0, fmt.Errorf("environment busy or unavailable (%s): %w", paths.Root, err)
	}
	defer lock.Close()
	if info, err := os.Lstat(filepath.Join(paths.State, "operation.lock")); err != nil || !info.Mode().IsRegular() {
		return 0, fmt.Errorf("operation lock is not an owned regular file")
	}
	if c.Prepare != nil {
		if err = c.Prepare(paths); err != nil {
			return 0, fmt.Errorf("portable preparation failed: %w", err)
		}
	}
	statusPath := filepath.Join(paths.State, "operation.json")
	var previous operationStatus
	if err := readJSON(paths.Root, statusPath, &previous); err == nil {
		if previous.SchemaVersion != 1 {
			return 0, fmt.Errorf("unsupported operation-state schema")
		}
		if previous.Status == "running" && c.Diagnostic != nil {
			fmt.Fprintln(c.Diagnostic, "portscoop: previous package operation was interrupted; inspect package status before further changes")
		}
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	status := operationStatus{SchemaVersion: 1, Operation: c.Operation, Status: "running"}
	if err := writeJSON(statusPath, status); err != nil {
		return 0, err
	}
	code, executionError := execute()
	status.Status = "complete"
	if executionError != nil || code != 0 {
		status.Status = "failed"
		status.Detail = fmt.Sprintf("status=%d error=%v", code, executionError)
	}
	if c.Capture != nil {
		if err = c.Capture(paths); err != nil {
			status.Status = "inspection-required"
			status.Detail = err.Error()
			_ = writeJSON(statusPath, status)
			return code, fmt.Errorf("portable state requires inspection after operation: %w", errors.Join(err, executionError))
		}
	}
	if err := writeJSON(statusPath, status); err != nil {
		return code, err
	}
	return code, executionError
}

type operationStatus struct {
	SchemaVersion int    `json:"schema_version"`
	Operation     string `json:"operation"`
	Status        string `json:"status"`
	Detail        string `json:"detail,omitempty"`
}

func validateLayout(paths environment.Paths) error {
	for _, path := range []string{paths.Control, paths.State, paths.Scoop, paths.Apps, paths.Buckets, paths.Cache, paths.Persist, paths.Shims, paths.Global, paths.Temp, filepath.Dir(paths.Adapter), filepath.Dir(filepath.Dir(paths.Engine))} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("managed layout directory must not be redirected: %s", path)
		}
		if err := containedParent(paths.Root, filepath.Join(path, "probe")); err != nil {
			return err
		}
	}
	// Git bucket removal must never recurse into a redirected repository.
	if _, err := os.Stat(paths.Buckets); err == nil {
		return filepath.WalkDir(paths.Buckets, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return fmt.Errorf("bucket contains an unsupported reparse point: %s", path)
			}
			return nil
		})
	}
	return nil
}

func containedParent(root, path string) error {
	parent := filepath.Dir(path)
	for {
		_, err := os.Lstat(parent)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return err
		}
		parent = next
	}
	parent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || filepath.IsAbs(relative) || (len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator)) {
		return fmt.Errorf("managed target escapes portable root: %s", path)
	}
	return nil
}
