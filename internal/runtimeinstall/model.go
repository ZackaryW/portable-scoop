// Package runtimeinstall defines distribution ownership and runtime coordination.
// It has no package-manager or application execution dependency.
package runtimeinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"portscoop/internal/atomicfile"
	"portscoop/internal/environment"
	"portscoop/internal/metadata"
)

const DescriptorName = "runtime-update.json"
const ReceiptName = ".portscoop/state/runtime.json"
const HostControl = ".portscoop-host"
const StoreControl = ".portscoop/state/runtime-update"

var Launchers = []string{"portscoop.exe", "psp.exe", "pspe.exe"}
var DigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Contract struct {
	Schema   int    `json:"schema_version"`
	Protocol int    `json:"protocol_major"`
	Upstream string `json:"upstream_revision"`
	Patch    string `json:"patch_revision"`
}
type Descriptor struct {
	Schema    int        `json:"schema_version"`
	Protocol  int        `json:"update_protocol"`
	Version   string     `json:"version"`
	Platform  string     `json:"platform"`
	Kinds     []string   `json:"target_kinds"`
	Launchers []string   `json:"launchers"`
	Backends  []Contract `json:"backends"`
}
type Release struct {
	SchemaVersion int               `json:"schema_version"`
	PatchRevision string            `json:"patch_revision"`
	Artifacts     map[string]string `json:"artifacts"`
}
type Receipt struct {
	Schema    int               `json:"schema_version"`
	ID        string            `json:"id"`
	Version   string            `json:"version"`
	Artifacts map[string]string `json:"artifacts"`
}
type HostIdentity struct {
	Schema int    `json:"schema_version"`
	ID     string `json:"id"`
}
type Target struct {
	Root    string `json:"root"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

func (t Target) Control() string {
	if t.Kind == "store" {
		return filepath.Join(t.Root, StoreControl)
	}
	return filepath.Join(t.Root, HostControl)
}

// CheckPath rejects reparse points at every existing component, including root.
// Only normalized, portable relative names may become replacement operations.
func CheckPath(root, relative string) (string, error) {
	if !metadata.RelativePath(relative) || strings.ContainsAny(relative, "\r\n\t\"<>|?*") {
		return "", fmt.Errorf("invalid owned path %q", relative)
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) || strings.HasPrefix(root, `\\`) || filepath.Dir(root) == root {
		return "", fmt.Errorf("a local installation folder is required")
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return "", fmt.Errorf("redirected update path: %s", p)
		}
		if strings.EqualFold(p, root) {
			break
		}
		if filepath.Dir(p) == p {
			return "", fmt.Errorf("path escaped installation")
		}
	}
	return path, nil
}
func Read(root, relative string, v any) error {
	p, err := CheckPath(root, relative)
	if err != nil {
		return err
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 16*1024*1024+1))
	if err != nil {
		return err
	}
	if len(b) > 16*1024*1024 {
		return fmt.Errorf("metadata exceeds bound")
	}
	return metadata.Decode(b, v)
}
func Write(root, relative string, v any) error {
	p, err := CheckPath(root, relative)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	if _, err = CheckPath(root, relative); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), "publish-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return atomicfile.Replace(f.Name(), p)
}
func Hash(root, relative string) (string, error) {
	p, err := CheckPath(root, relative)
	if err != nil {
		return "", err
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular owned file: %s", relative)
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (d Descriptor) Validate() error {
	if d.Schema != 1 || d.Protocol != 1 || d.Version == "" || d.Platform != "windows-amd64" || len(d.Kinds) != 2 || d.Kinds[0] != "host" || d.Kinds[1] != "store" || len(d.Launchers) != 3 || len(d.Backends) == 0 {
		return fmt.Errorf("unsupported runtime update descriptor")
	}
	for i, n := range Launchers {
		if d.Launchers[i] != n {
			return fmt.Errorf("invalid descriptor launcher set")
		}
	}
	for _, b := range d.Backends {
		if b.Schema != 1 || b.Protocol != 1 || len(b.Upstream) != 40 || b.Patch == "" {
			return fmt.Errorf("invalid backend update contract")
		}
	}
	return nil
}
func NewReceipt(id string, d Descriptor, r Release) Receipt {
	a := map[string]string{}
	for _, n := range Launchers {
		a[n] = r.Artifacts[n]
	}
	return Receipt{1, id, d.Version, a}
}
func VerifyReceipt(root string, id string) (Receipt, error) {
	var r Receipt
	if err := Read(root, ReceiptName, &r); err != nil {
		return r, fmt.Errorf("runtime receipt missing or invalid; deploy a complete release into a fresh folder: %w", err)
	}
	if r.Schema != 1 || r.ID != id || r.Version == "" || len(r.Artifacts) != 3 || r.Artifacts["psp.exe"] != r.Artifacts["portscoop.exe"] {
		return r, fmt.Errorf("invalid Store runtime ownership")
	}
	for _, n := range Launchers {
		h, err := Hash(root, n)
		if err != nil {
			return r, err
		}
		if !DigestPattern.MatchString(r.Artifacts[n]) || h != r.Artifacts[n] {
			return r, fmt.Errorf("Store launcher ownership drift: %s", n)
		}
	}
	return r, nil
}
func Select(executable string, verify bool) (Target, error) {
	abs, err := filepath.Abs(executable)
	if err != nil {
		return Target{}, err
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return Target{}, err
	}
	t := Target{Root: root, Kind: "host"}
	if _, err := os.Lstat(filepath.Join(root, ".portscoop")); err == nil {
		var id environment.Identity
		if err = Read(root, environment.MarkerRelative, &id); err != nil {
			return t, err
		}
		if err = id.Validate(); err != nil {
			return t, err
		}
		t.Kind, t.ID = "store", id.ID
		var r Receipt
		if verify {
			r, err = VerifyReceipt(root, id.ID)
		} else {
			err = Read(root, ReceiptName, &r)
		}
		if err != nil {
			return t, err
		}
		if r.Schema != 1 || r.ID != id.ID {
			return t, fmt.Errorf("invalid runtime receipt")
		}
		t.Version = r.Version
		return t, nil
	} else if !os.IsNotExist(err) {
		return t, err
	}
	var h HostIdentity
	if err = Read(root, HostControl+"/identity.json", &h); err == nil {
		if h.Schema != 1 || !environment.ValidID(h.ID) {
			return t, fmt.Errorf("invalid Host identity")
		}
		t.ID = h.ID
	} else if !os.IsNotExist(err) {
		return t, err
	}
	if verify {
		_, d, e := VerifyDistribution(root, true)
		err = e
		t.Version = d.Version
	} else {
		var d Descriptor
		err = Read(root, DescriptorName, &d)
		if err == nil {
			err = d.Validate()
		}
		t.Version = d.Version
	}
	return t, err
}
