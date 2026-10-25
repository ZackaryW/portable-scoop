// Package localipc implements bounded, invocation-private file mailboxes.
// Each directory is atomically created with a protected current-user-only DACL.
// Control JSON is separate from application streams; no listener is opened.
package localipc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"portscoop/internal/metadata"
)

const MaxMessage = 1024 * 1024
const StartupTimeout = 15 * time.Second

type Endpoint struct {
	directory, send, receive    string
	writeSequence, readSequence int
	owner                       bool
}

var directoryName = regexp.MustCompile(`^portscoop-execution-[0-9a-f]{64}$`)

// Windows can briefly deny access while another process inspects a newly
// published file. Retry only sharing/byte-range locks, within this operation's
// deadline. This retries transport I/O, never an invocation or application.
func retrySharing(ctx context.Context, action func() error) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := action()
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func currentSID() (*windows.SID, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid.Copy()
}

func New(parent string) (*Endpoint, error) {
	if !filepath.IsAbs(parent) || strings.HasPrefix(parent, `\\`) {
		return nil, fmt.Errorf("execution transport requires a local absolute parent")
	}
	sid, err := currentSID()
	if err != nil {
		return nil, err
	}
	sd, err := windows.SecurityDescriptorFromString("O:" + sid.String() + "D:P(A;OICI;FA;;;" + sid.String() + ")")
	if err != nil {
		return nil, err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	dir := filepath.Join(parent, "portscoop-execution-"+hex.EncodeToString(nonce[:]))
	ptr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, err
	}
	if err := windows.CreateDirectory(ptr, &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}); err != nil {
		return nil, err
	}
	e := &Endpoint{directory: dir, send: "server", receive: "client", owner: true}
	if err := validateDirectory(dir); err != nil {
		_ = os.Remove(dir)
		return nil, err
	}
	return e, nil
}

func Open(directory string) (*Endpoint, error) {
	if err := validateDirectory(directory); err != nil {
		return nil, err
	}
	return &Endpoint{directory: directory, send: "client", receive: "server"}, nil
}
func (e *Endpoint) Directory() string { return e.directory }

func validateDirectory(dir string) error {
	if !filepath.IsAbs(dir) || strings.HasPrefix(dir, `\\`) || !directoryName.MatchString(filepath.Base(dir)) {
		return fmt.Errorf("invalid private execution transport directory")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("execution transport is redirected")
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	sid, err := currentSID()
	if err != nil {
		return err
	}
	return validateSecurityDescriptor(sd, sid)
}

func validateSecurityDescriptor(sd *windows.SECURITY_DESCRIPTOR, sid *windows.SID) error {
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !windows.EqualSid(owner, sid) {
		return fmt.Errorf("execution transport has another owner")
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("execution transport DACL is not protected")
	}
	// SDDL can abbreviate the same SID (for example, the local administrator
	// as LA). Validate the actual ACE rather than its display representation.
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 1 {
		return fmt.Errorf("execution transport DACL is not current-user-only")
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		return fmt.Errorf("inspect execution transport DACL: %w", err)
	}
	const fullControl windows.ACCESS_MASK = 0x001f01ff // FILE_ALL_ACCESS
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
		ace.Header.AceFlags != windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE ||
		ace.Mask != fullControl {
		return fmt.Errorf("execution transport DACL is not current-user-only")
	}
	trustee := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !trustee.IsValid() || !windows.EqualSid(trustee, sid) {
		return fmt.Errorf("execution transport DACL is not current-user-only")
	}
	return nil
}

func (e *Endpoint) Write(ctx context.Context, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > MaxMessage {
		return fmt.Errorf("execution message exceeds size bound")
	}
	path := filepath.Join(e.directory, fmt.Sprintf("%s-%04d.json", e.send, e.writeSequence))
	file, err := os.CreateTemp(e.directory, e.send+"-*.tmp")
	if err != nil {
		return fmt.Errorf("execution transport disconnected: %w", err)
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := retrySharing(ctx, func() error { return os.Rename(file.Name(), path) }); err != nil {
		return err
	}
	e.writeSequence++
	return nil
}

func (e *Endpoint) Read(ctx context.Context, value any) error {
	path := filepath.Join(e.directory, fmt.Sprintf("%s-%04d.json", e.receive, e.readSequence))
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var info os.FileInfo
		err := retrySharing(ctx, func() error { var err error; info, err = os.Lstat(path); return err })
		if err == nil {
			if !info.Mode().IsRegular() || info.Size() > MaxMessage {
				return fmt.Errorf("invalid execution transport message")
			}
			var data []byte
			err := retrySharing(ctx, func() error {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()
				data, err = io.ReadAll(io.LimitReader(file, MaxMessage+1))
				return err
			})
			if err != nil {
				return err
			}
			if len(data) > MaxMessage {
				return fmt.Errorf("execution message exceeds size bound")
			}
			if err := metadata.Decode(data, value); err != nil {
				return fmt.Errorf("invalid execution message: %w", err)
			}
			if err := retrySharing(ctx, func() error { return os.Remove(path) }); err != nil {
				return err
			}
			e.readSequence++
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		if _, err := os.Stat(e.directory); err != nil {
			return fmt.Errorf("execution transport disconnected: %w", err)
		}
		if _, err := os.Stat(filepath.Join(e.directory, e.receive+".closed")); err == nil {
			return fmt.Errorf("execution peer disconnected")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (e *Endpoint) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e.owner {
		// The exact absolute random directory was created by New and rechecked.
		return retrySharing(ctx, func() error {
			if err := validateDirectory(e.directory); err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			return os.RemoveAll(e.directory)
		})
	}
	return retrySharing(ctx, func() error {
		return os.WriteFile(filepath.Join(e.directory, e.send+".closed"), []byte("closed"), 0600)
	})
}
