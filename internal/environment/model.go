package environment

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"portscoop/internal/metadata"
)

const SchemaVersion = 1
const ProtocolMajor = 1
const MarkerRelative = ".portscoop/environment.json"
const EngineEntryRelative = "scoop/apps/scoop/current/bin/scoop.ps1"
const AdapterRelative = ".portscoop/backend/entry.ps1"

var identityPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Identity struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	ProtocolMajor int    `json:"protocol_major"`
	Ready         bool   `json:"ready"`
}

func ValidID(id string) bool { return identityPattern.MatchString(id) }

func NewIdentity(name string) (Identity, error) {
	var bits [16]byte
	if _, err := rand.Read(bits[:]); err != nil {
		return Identity{}, err
	}
	bits[6] = (bits[6] & 0x0f) | 0x40
	bits[8] = (bits[8] & 0x3f) | 0x80
	id := fmt.Sprintf("%x-%x-%x-%x-%x", bits[0:4], bits[4:6], bits[6:8], bits[8:10], bits[10:16])
	i := Identity{SchemaVersion, id, name, ProtocolMajor, true}
	return i, i.Validate()
}

func (i Identity) Validate() error {
	if i.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported environment schema %d", i.SchemaVersion)
	}
	if !ValidID(i.ID) {
		return fmt.Errorf("invalid installation identity")
	}
	if strings.TrimSpace(i.Name) == "" || strings.ContainsAny(i.Name, "\x00\r\n") {
		return fmt.Errorf("invalid environment display name")
	}
	if i.ProtocolMajor != ProtocolMajor {
		return fmt.Errorf("unsupported delegation protocol %d", i.ProtocolMajor)
	}
	if !i.Ready {
		return fmt.Errorf("environment initialization is incomplete")
	}
	return nil
}

func DecodeIdentity(data []byte) (Identity, error) {
	var i Identity
	if err := metadata.Decode(data, &i); err != nil {
		return i, fmt.Errorf("environment metadata: %w", err)
	}
	return i, i.Validate()
}

// Paths is constructed from one absolute root; identity metadata cannot override
// executable or state paths independently.
type Paths struct {
	Root, Control, State, Scoop, Config, Engine, Adapter string
	Apps, Buckets, Cache, Persist, Shims, Global, Temp   string
}

func PathsAt(root string) (Paths, error) {
	if !filepath.IsAbs(root) {
		return Paths{}, fmt.Errorf("environment root must be absolute")
	}
	root = filepath.Clean(root)
	if filepath.Dir(root) == root {
		return Paths{}, fmt.Errorf("a volume root cannot be an environment")
	}
	s := filepath.Join(root, "scoop")
	c := filepath.Join(root, ".portscoop")
	return Paths{
		Root: root, Control: c, State: filepath.Join(c, "state"), Scoop: s, Config: filepath.Join(s, "config.json"),
		Engine: filepath.Join(root, filepath.FromSlash(EngineEntryRelative)), Adapter: filepath.Join(root, filepath.FromSlash(AdapterRelative)),
		Apps: filepath.Join(s, "apps"), Buckets: filepath.Join(s, "buckets"), Cache: filepath.Join(s, "cache"),
		Persist: filepath.Join(s, "persist"), Shims: filepath.Join(s, "shims"), Global: filepath.Join(s, "global"), Temp: filepath.Join(s, "tmp"),
	}, nil
}

type BackendMetadata struct {
	SchemaVersion    int               `json:"schema_version"`
	ProtocolMajor    int               `json:"protocol_major"`
	UpstreamRevision string            `json:"upstream_revision"`
	PatchRevision    string            `json:"patch_revision"`
	Artifacts        map[string]string `json:"artifacts"`
}

func (b BackendMetadata) Validate() error {
	if b.SchemaVersion != SchemaVersion || b.ProtocolMajor != ProtocolMajor {
		return fmt.Errorf("incompatible backend metadata")
	}
	if !revisionPattern.MatchString(b.UpstreamRevision) || b.PatchRevision == "" {
		return fmt.Errorf("invalid backend revision")
	}
	if len(b.Artifacts) == 0 {
		return fmt.Errorf("backend has no verified artifacts")
	}
	seen := make(map[string]bool)
	for path, hash := range b.Artifacts {
		canonical := strings.ToLower(strings.ReplaceAll(path, `\`, "/"))
		if !metadata.RelativePath(path) || !digestPattern.MatchString(hash) || seen[canonical] {
			return fmt.Errorf("invalid backend artifact %q", path)
		}
		seen[canonical] = true
	}
	if !seen[EngineEntryRelative] || !seen[AdapterRelative] {
		return fmt.Errorf("backend must verify engine and adapter entry points")
	}
	return nil
}

func DecodeBackend(data []byte) (BackendMetadata, error) {
	var b BackendMetadata
	if err := metadata.Decode(data, &b); err != nil {
		return b, fmt.Errorf("backend metadata: %w", err)
	}
	return b, b.Validate()
}

// Context is invocation-local. It is passed explicitly to the backend runner.
type Context struct {
	Identity         Identity
	Paths            Paths
	Backend          BackendMetadata
	PowerShell       string
	WorkingDirectory string
}
