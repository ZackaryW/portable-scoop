package selfupdate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/semver"
)

func normalize(v string) string { return "v" + strings.TrimPrefix(v, "v") }
func (s Service) Run(ctx context.Context, exe string, o Options) (Result, error) {
	n := 0
	for _, v := range []bool{o.Check, o.Status, o.Recover} {
		if v {
			n++
		}
	}
	if n > 1 || ((o.Status || o.Recover) && o.Version != "") {
		return Result{}, fmt.Errorf("combine --version only with an update or --check")
	}
	t, err := s.Files.Select(exe, !o.Status && !o.Recover)
	if err != nil {
		return Result{}, err
	}
	if o.Status {
		return s.Files.Status(t)
	}
	if o.Recover {
		tx, err := s.Files.Load(t)
		if err != nil {
			return Result{}, err
		}
		// Worker serializes recovery and validates backups before any mutation.
		return s.Worker.Handoff(ctx, tx, true)
	}
	if !semver.IsValid(normalize(t.Version)) && o.Version == "" {
		return Result{}, fmt.Errorf("development version %s requires an explicit --version", t.Version)
	}
	r, err := s.Discovery.Resolve(ctx, o.Version)
	if err != nil {
		return Result{}, err
	}
	current := normalize(t.Version)
	if semver.IsValid(current) {
		comparison := semver.Compare(current, r.Tag)
		if comparison > 0 && o.Version != "" {
			return Result{}, fmt.Errorf("downgrading %s to %s is unsupported", current, r.Tag)
		}
		if comparison >= 0 {
			return Result{0, fmt.Sprintf("%s %s is already current (candidate %s).", t.Kind, t.Version, r.Tag)}, nil
		}
	}
	if o.Check {
		return Result{0, fmt.Sprintf("%s: %s -> %s available. Candidate payload and backend compatibility have not been verified.", t.Kind, t.Version, r.Tag)}, nil
	}
	t, err = s.Files.Ensure(t)
	if err != nil {
		return Result{}, err
	}
	owner, err := s.Locks.Acquire(ctx, t, "update", true)
	if err != nil {
		return Result{}, err
	}
	defer owner.Close()
	if err = s.Files.Pending(t); err == nil {
		return Result{}, fmt.Errorf("an update is already pending; use --status or --recover")
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}
	use, err := s.Locks.Acquire(ctx, t, "runtime", false)
	if err != nil {
		return Result{}, err
	}
	defer use.Close()
	// Revalidate owned files under coordination; a concurrent update may have
	// completed during discovery.
	verified, err := s.Files.Select(exe, true)
	if err != nil {
		return Result{}, err
	}
	if verified.ID != t.ID || verified.Version != t.Version {
		return Result{}, fmt.Errorf("installation changed during discovery; retry")
	}
	tx, err := s.Files.Stage(ctx, t, r, s.Downloads, s.Clock.Now())
	if err != nil {
		return Result{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = s.Files.Discard(tx)
		}
	}()
	if err = s.Files.Prepare(tx); err != nil {
		return Result{}, err
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	if err = s.Files.Publish(tx); err != nil {
		return Result{}, err
	}
	published = true
	result, err := s.Worker.Handoff(ctx, tx, false)
	if err != nil {
		// Handoff errors are returned only before commit permission is accepted.
		// Uncertain ownership is a Result (code 2), and deliberately retains the gate.
		if e := s.Files.Abandon(tx, err); e != nil {
			return Result{}, fmt.Errorf("%w; startup cleanup requires recovery: %v", err, e)
		}
	}
	// After pending publication, retain all evidence even if acknowledgement is
	// uncertain. Explicit recovery, never an automatic second worker, resolves it.
	return result, err
}
