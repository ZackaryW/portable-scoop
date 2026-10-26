package scoop

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"portscoop/internal/archive"
	"portscoop/internal/compatibility"
	"portscoop/internal/environment"
	"portscoop/internal/runner"
	"portscoop/internal/runtimeinstall"
)

// ExtractFrozen is a private backend operation; the invoking install already
// owns the Store operation lock. It never accepts arbitrary source/target paths.
func ExtractFrozen(ctx context.Context, executable, requestPath, app string) error {
	target, err := runtimeinstall.Select(executable, true)
	if err != nil {
		return err
	}
	if target.Kind != "store" {
		return fmt.Errorf("extraction requires a verified Store runtime")
	}
	paths, err := environment.PathsAt(target.Root)
	if err != nil {
		return err
	}
	if _, err = Verify(paths); err != nil {
		return err
	}
	return extractRequest(ctx, paths, target.ID, requestPath, app)
}
func extractRequest(ctx context.Context, paths environment.Paths, id, requestPath, app string) error {
	if !PackageName(app) || !archive.SafePath(app) || strings.ContainsAny(app, `/\`) {
		return fmt.Errorf("invalid extraction package")
	}
	requestPath, err := filepath.Abs(requestPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Dir(requestPath), paths.Temp) || !strings.HasPrefix(filepath.Base(requestPath), "portscoop-request-") {
		return fmt.Errorf("extraction request outside owned transport")
	}
	if err = archive.Confined(paths.Root, requestPath); err != nil {
		return err
	}
	var request runner.Request
	rel, _ := filepath.Rel(paths.Root, requestPath)
	if err = runtimeinstall.Read(paths.Root, rel, &request); err != nil {
		return err
	}
	if request.SchemaVersion != 1 || request.ExpectedID != id {
		return fmt.Errorf("extraction request identity mismatch")
	}
	op, err := Classify(request.Arguments)
	if err != nil {
		return err
	}
	if op.Name != "install" && op.Name != "update" {
		return fmt.Errorf("extraction is not an install/update operation")
	}
	var manifests map[string]FrozenManifest
	if err = json.Unmarshal(request.Manifests, &manifests); err != nil {
		return err
	}
	frozen, ok := manifests[strings.ToLower(app)]
	if !ok {
		return fmt.Errorf("package absent from frozen extraction plan")
	}
	compiled, err := compatibility.Compile(frozen.Manifest)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(compiled.Downloads, frozen.Downloads) {
		return fmt.Errorf("frozen extraction plan differs from manifest")
	}
	var version string
	if err = json.Unmarshal(compiled.Manifest["version"], &version); err != nil {
		return err
	}
	physical := filepath.Join(paths.Apps, app, version)
	if err = archive.Confined(paths.Root, physical); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(paths.Temp, "portscoop-extraction-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, d := range compiled.Downloads {
		source := filepath.Join(physical, d.Name)
		if err = archive.Confined(paths.Root, source); err != nil {
			return err
		}
		if err = verifyDownload(ctx, source, d.Hash); err != nil {
			return err
		}
		if d.Format == "file" {
			continue
		}
		resolveNSIS := func() (archive.NSISReader, error) {
			helper, err := Helper(paths, "7zip")
			if err != nil {
				return nil, err
			}
			// Fixed DLL directory and a minimal process-local environment avoid
			// consulting host tools or user extraction configuration.
			return archive.NativeNSIS{Executable: helper, Environment: []string{
				"SystemRoot=" + os.Getenv("SystemRoot"), "PATH=" + filepath.Dir(helper),
				"TEMP=" + paths.Temp, "TMP=" + paths.Temp,
			}}, nil
		}
		if err = archive.Extract(ctx, paths.Root, archive.Request{Source: source, Destination: stage, Format: d.Format, ExtractDir: d.ExtractDir, ExtractTo: d.ExtractTo, NSIS: resolveNSIS}); err != nil {
			return fmt.Errorf("extract %s: %w", d.Name, err)
		}
	}
	if err = archive.Publish(ctx, paths.Root, stage, physical); err != nil {
		return err
	}
	for _, d := range compiled.Downloads {
		if d.Format != "file" {
			if err = os.Remove(filepath.Join(physical, d.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}
func verifyDownload(ctx context.Context, path, digest string) error {
	algorithm, want, found := strings.Cut(digest, ":")
	if !found {
		algorithm, want = "sha256", digest
	}
	var h hash.Hash
	switch algorithm {
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	case "sha1":
		h = sha1.New()
	case "md5":
		h = md5.New()
	default:
		return fmt.Errorf("unsupported download digest")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("download is not a regular file")
	}
	buffer := make([]byte, 65536)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := f.Read(buffer)
		h.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return fmt.Errorf("download digest changed before extraction")
	}
	return nil
}
