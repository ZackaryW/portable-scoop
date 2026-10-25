// Package selfupdate maintains Portscoop runtimes without dispatching Scoop.
package selfupdate

import (
	"context"
	"io"
	"portscoop/internal/runtimeinstall"
	"time"
)

type Options struct {
	Check, Status, Recover bool
	Version                string
}
type Result struct {
	Code    int
	Message string
}
type Release struct{ Tag, Archive, Checksum string }
type Discovery interface {
	Resolve(context.Context, string) (Release, error)
}
type Downloader interface {
	Download(context.Context, string, io.Writer, int64) error
}
type Clock interface{ Now() time.Time }
type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type Lease interface{ Close() error }
type Locker interface {
	Acquire(context.Context, runtimeinstall.Target, string, bool) (Lease, error)
}
type NativeLocker struct{}

func (NativeLocker) Acquire(c context.Context, t runtimeinstall.Target, n string, x bool) (Lease, error) {
	return runtimeinstall.Lock(c, t, n, x)
}

type Worker interface {
	Handoff(context.Context, *Transaction, bool) (Result, error)
}

// Storage is the filesystem boundary, independently injectable from transport,
// time, coordination and process lifetime. Implementations must preserve the
// containment and ownership contracts of each phase.
type Storage interface {
	Select(string, bool) (runtimeinstall.Target, error)
	Ensure(runtimeinstall.Target) (runtimeinstall.Target, error)
	Stage(context.Context, runtimeinstall.Target, Release, Downloader, time.Time) (*Transaction, error)
	Prepare(*Transaction) error
	Load(runtimeinstall.Target) (*Transaction, error)
	Status(runtimeinstall.Target) (Result, error)
	Publish(*Transaction) error
	Discard(*Transaction) error
	Pending(runtimeinstall.Target) error
	Abandon(*Transaction, error) error
}
type Service struct {
	Discovery  Discovery
	Downloads  Downloader
	Files      Storage
	Clock      Clock
	Locks      Locker
	Worker     Worker
	controller *runtimeinstall.Target
	selectedID string
}
type Operation struct {
	Path string `json:"path"`
	Old  string `json:"old,omitempty"`
	New  string `json:"new,omitempty"`
}
type Transaction struct {
	Controller    *runtimeinstall.Target `json:"controller,omitempty"`
	Schema        int                    `json:"schema_version"`
	Target        runtimeinstall.Target  `json:"target"`
	Directory     string                 `json:"directory"`
	Nonce         string                 `json:"nonce"`
	Version       string                 `json:"version"`
	Created       time.Time              `json:"created"`
	WorkerHash    string                 `json:"worker_hash"`
	BackendHash   string                 `json:"backend_hash,omitempty"`
	ParentPID     uint32                 `json:"parent_pid"`
	ParentCreated uint64                 `json:"parent_created"`
	ParentImage   string                 `json:"parent_image"`
	Operations    []Operation            `json:"operations"`
}
type Journal struct {
	Schema  int    `json:"schema_version"`
	ID      string `json:"id"`
	Nonce   string `json:"nonce"`
	Phase   string `json:"phase"`
	Applied int    `json:"applied"`
	Detail  string `json:"detail,omitempty"`
}

func Default() Service {
	net := NewGitHub()
	return Service{Discovery: net, Downloads: net, Files: Disk{}, Clock: realClock{}, Locks: NativeLocker{}, Worker: WindowsWorker{}}
}
