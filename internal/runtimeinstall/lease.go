package runtimeinstall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"portscoop/internal/filelock"
	"time"
)

const BusyTimeout = 5 * time.Second

type Pending struct {
	Schema      int    `json:"schema_version"`
	ID          string `json:"id"`
	Transaction string `json:"transaction"`
	Nonce       string `json:"nonce"`
}

func ControlRelative(t Target) string {
	if t.Kind == "store" {
		return StoreControl
	}
	return HostControl
}
func Lock(ctx context.Context, t Target, name string, exclusive bool) (*filelock.Lock, error) {
	rel := ControlRelative(t) + "/" + name + ".lock"
	p, err := CheckPath(t.Root, rel)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return nil, err
	}
	if _, err = CheckPath(t.Root, rel); err != nil {
		return nil, err
	}
	c, cancel := context.WithTimeout(ctx, BusyTimeout)
	defer cancel()
	return filelock.AcquireMode(c, p, exclusive)
}
func ReadPending(t Target) (Pending, error) {
	var p Pending
	err := Read(t.Root, ControlRelative(t)+"/pending.json", &p)
	if err == nil && (p.Schema != 1 || p.ID != t.ID || !DigestPattern.MatchString(p.Nonce) || filepath.Base(p.Transaction) != p.Transaction) {
		err = fmt.Errorf("invalid runtime pending identity")
	}
	return p, err
}

// Use is called by both executable frontends, including delegated entry points.
// Unmanaged development binaries remain runnable, but cannot self-update.
func Use(ctx context.Context, executable string) (*filelock.Lock, error) {
	root, err := filepath.EvalSymlinks(filepath.Dir(executable))
	if err != nil {
		return nil, err
	}
	_, storeErr := os.Lstat(filepath.Join(root, ".portscoop"))
	_, hostErr := os.Lstat(filepath.Join(root, DescriptorName))
	if os.IsNotExist(storeErr) && os.IsNotExist(hostErr) {
		return nil, nil
	}
	t, err := Select(executable, false)
	if err != nil {
		return nil, err
	}
	if t.Kind == "host" {
		if t.ID == "" {
			if _, _, err = VerifyDistribution(t.Root, true); err != nil {
				return nil, err
			}
		}
		t, err = EnsureHost(t)
		if err != nil {
			return nil, err
		}
	}
	l, err := Lock(ctx, t, "runtime", false)
	if err != nil {
		return nil, err
	}
	if p, e := ReadPending(t); e == nil {
		l.Close()
		return nil, fmt.Errorf("runtime update %s is pending or interrupted; use self-update --status or --recover", p.Transaction)
	} else if !os.IsNotExist(e) {
		l.Close()
		return nil, e
	}
	return l, nil
}
