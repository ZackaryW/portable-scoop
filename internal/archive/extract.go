// Package archive extracts supported containers through bounded, staged writes.
// It never executes payloads. Only recognized NSIS content may use an explicitly
// supplied, verified native reader; other formats retain their Go readers.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bodgit/sevenzip"
	"portscoop/internal/metadata"
)

type Limits struct {
	Files                   int
	TotalBytes, MemberBytes int64
}

var DefaultLimits = Limits{Files: 100000, TotalBytes: 8 << 30, MemberBytes: 4 << 30}

type Request struct {
	Source, Destination, Format, ExtractDir, ExtractTo string
	Limits                                             Limits
	// NSIS resolves the verified Store helper lazily, only for NSIS content.
	NSIS func() (NSISReader, error)
}

func SafePath(s string) bool {
	if !metadata.RelativePath(s) || len(s) > 1024 || strings.ContainsAny(s, "<>\"|?*`\x00\r\n\t") {
		return false
	}
	for _, c := range s {
		if c < 32 || c == 127 {
			return false
		}
	}
	for _, p := range strings.Split(strings.ReplaceAll(s, `\`, "/"), "/") {
		b := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		if b == "CON" || b == "PRN" || b == "AUX" || b == "NUL" || (len(b) == 4 && (strings.HasPrefix(b, "COM") || strings.HasPrefix(b, "LPT")) && b[3] >= '0' && b[3] <= '9') {
			return false
		}
	}
	return true
}

// Confined forbids all reparse ancestors, including the Store root itself.
func Confined(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || (rel != "." && !SafePath(rel)) {
		return fmt.Errorf("archive path escapes Store")
	}
	for cursor := target; ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("archive path contains a reparse point")
		}
		if err == nil && cursor != target && !info.IsDir() {
			return fmt.Errorf("archive path ancestor is not a directory")
		}
		if strings.EqualFold(cursor, root) {
			break
		}
		if filepath.Dir(cursor) == cursor {
			return fmt.Errorf("archive root not reached")
		}
	}
	return nil
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

type ctxReaderAt struct {
	ctx context.Context
	r   io.ReaderAt
}

func (r ctxReaderAt) ReadAt(p []byte, o int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.ReadAt(p, o)
}

type entry struct {
	name string
	dir  bool
}
type staging struct {
	ctx     context.Context
	root    string
	limits  Limits
	bytes   int64
	count   int
	entries map[string]entry
}

func (s *staging) add(name string, mode fs.FileMode, size int64, r io.Reader) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, `\`, "/")
	for strings.HasPrefix(name, "./") {
		name = strings.TrimPrefix(name, "./")
	}
	name = strings.TrimSuffix(name, "/")
	if name == "." && mode.IsDir() {
		return nil
	}
	if !SafePath(name) || (!mode.IsDir() && !mode.IsRegular()) {
		return fmt.Errorf("unsafe archive member %q", name)
	}
	s.count++
	if s.count > s.limits.Files || size < 0 || size > s.limits.MemberBytes || size > s.limits.TotalBytes-s.bytes {
		return fmt.Errorf("archive resource limit exceeded")
	}
	s.bytes += size
	// Track implicit directories too: Foo/a and foo/b are a case collision.
	parts := strings.Split(name, "/")
	for i := range parts {
		n := strings.Join(parts[:i+1], "/")
		key := strings.ToLower(n)
		dir := i < len(parts)-1 || mode.IsDir()
		if old, ok := s.entries[key]; ok {
			if old.name != n || !old.dir || !dir {
				return fmt.Errorf("archive member collision %q", n)
			}
		} else {
			s.entries[key] = entry{n, dir}
		}
	}
	target := filepath.Join(s.root, filepath.FromSlash(name))
	if mode.IsDir() {
		return os.MkdirAll(target, 0700)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, io.LimitReader(ctxReader{s.ctx, r}, size+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if n != size {
		return fmt.Errorf("archive member size differs: %s", name)
	}
	return closeErr
}
func Extract(ctx context.Context, root string, q Request) error {
	if q.Limits.Files == 0 {
		q.Limits = DefaultLimits
	}
	if q.Limits.Files < 1 || q.Limits.MemberBytes < 1 || q.Limits.TotalBytes < 1 {
		return fmt.Errorf("invalid archive limits")
	}
	for _, p := range []string{q.ExtractDir, q.ExtractTo} {
		if p != "" && p != "." && !SafePath(p) {
			return fmt.Errorf("unsafe extraction selector")
		}
	}
	if err := Confined(root, q.Source); err != nil {
		return err
	}
	if err := Confined(root, q.Destination); err != nil {
		return err
	}
	fi, err := os.Stat(q.Destination)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("extraction destination must exist")
	}
	f, err := os.Open(q.Source)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<30 {
		return fmt.Errorf("invalid or oversized archive source")
	}
	stage, err := os.MkdirTemp(root, ".portscoop-extract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	s := staging{ctx: ctx, root: stage, limits: q.Limits, entries: map[string]entry{}}
	// Scoop uses #/dl.7z as a generic archive hint, including ZIP payloads.
	// Recognize supported content rather than assuming all such files are 7z.
	if q.Format == "7z" {
		var signature [4]byte
		if _, err := f.ReadAt(signature[:], 0); err == nil && (string(signature[:]) == "PK\x03\x04" || string(signature[:]) == "PK\x05\x06") {
			q.Format = "zip"
		} else if isNSIS(f) {
			q.Format = "nsis"
		}
	}
	switch q.Format {
	case "nsis":
		// The public format contract cannot select a native handler without
		// content recognition, including callers outside the frozen backend.
		if !isNSIS(f) || q.NSIS == nil {
			return fmt.Errorf("NSIS content requires a managed Store reader")
		}
		reader, err := q.NSIS()
		if err != nil {
			return err
		}
		if err = s.extractNSIS(q.Source, reader); err != nil {
			return err
		}
	case "zip":
		z, err := zip.NewReader(ctxReaderAt{ctx, f}, info.Size())
		if err != nil {
			return fmt.Errorf("ZIP contents: %w", err)
		}
		for _, m := range z.File {
			if m.ExternalAttrs&0x400 != 0 {
				return fmt.Errorf("reparse ZIP member")
			}
			r, err := m.Open()
			if err != nil {
				return err
			}
			err = s.add(m.Name, m.Mode(), int64(m.UncompressedSize64), r)
			ce := r.Close()
			if err != nil {
				return err
			}
			if ce != nil {
				return ce
			}
		}
	case "7z":
		// NewReader deliberately avoids OpenReader's automatic multi-volume lookup.
		z, err := sevenzip.NewReader(ctxReaderAt{ctx, f}, info.Size())
		if err != nil {
			return fmt.Errorf("7z contents: %w", err)
		}
		for _, m := range z.File {
			if m.Attributes&0x400 != 0 {
				return fmt.Errorf("reparse 7z member")
			}
			r, err := m.Open()
			if err != nil {
				return err
			}
			checksum := crc32.NewIEEE()
			err = s.add(m.Name, m.Mode(), int64(m.UncompressedSize), io.TeeReader(r, checksum))
			ce := r.Close()
			if err != nil {
				return err
			}
			if m.CRC32 != 0 && checksum.Sum32() != m.CRC32 {
				return fmt.Errorf("7z member CRC mismatch")
			}
			if ce != nil {
				return ce
			}
		}
	case "tar", "tar.gz":
		var r io.Reader = ctxReader{ctx, f}
		if q.Format == "tar.gz" {
			g, err := gzip.NewReader(r)
			if err != nil {
				return err
			}
			defer g.Close()
			r = g
		}
		tr := tar.NewReader(r)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA && h.Typeflag != tar.TypeDir {
				return fmt.Errorf("archive links and special members are unsupported")
			}
			if err = s.add(h.Name, h.FileInfo().Mode(), h.Size, tr); err != nil {
				return err
			}
		}
		// Read to EOF so gzip trailers/checksums are checked even after tar EOF.
		padding, err := io.Copy(io.Discard, io.LimitReader(r, (1<<20)+1))
		if padding > 1<<20 {
			return fmt.Errorf("excess archive trailer data")
		}
		if err != nil {
			return err
		}
	case "gzip":
		g, err := gzip.NewReader(ctxReader{ctx, f})
		if err != nil {
			return err
		}
		defer g.Close()
		// Gzip stores only a modulo-32-bit size, so spool with an explicit bound.
		raw, err := os.CreateTemp(stage, "gzip-")
		if err != nil {
			return err
		}
		defer raw.Close()
		n, err := io.Copy(raw, io.LimitReader(g, q.Limits.MemberBytes+1))
		if err != nil {
			return err
		}
		if n > q.Limits.MemberBytes {
			return fmt.Errorf("gzip member limit exceeded")
		}
		if _, err = raw.Seek(0, 0); err != nil {
			return err
		}
		if err = s.add(strings.TrimSuffix(filepath.Base(q.Source), filepath.Ext(q.Source)), 0600, n, raw); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported managed archive format %q", q.Format)
	}
	return s.publish(root, q)
}
func (s *staging) publish(root string, q Request) error {
	selector := strings.Trim(strings.ReplaceAll(q.ExtractDir, `\`, "/"), "/")
	if selector == "." {
		selector = ""
	}
	to := q.ExtractTo
	if to == "." {
		to = ""
	}
	type move struct {
		from, to string
		dir      bool
	}
	var moves []move
	if selector != "" {
		e, ok := s.entries[strings.ToLower(selector)]
		if !ok || !e.dir {
			return fmt.Errorf("extract_dir is not an archive directory")
		}
		selector = e.name
	}
	for _, e := range s.entries {
		name := e.name
		if selector != "" {
			if name == selector {
				continue
			}
			if !strings.HasPrefix(name, selector+"/") {
				continue
			}
			name = strings.TrimPrefix(name, selector+"/")
		}
		target := filepath.Join(q.Destination, filepath.FromSlash(to), filepath.FromSlash(name))
		if err := Confined(root, target); err != nil {
			return err
		}
		if info, err := os.Lstat(target); err == nil {
			if !e.dir || !info.IsDir() {
				return fmt.Errorf("extraction target already exists: %s", name)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		moves = append(moves, move{filepath.Join(s.root, filepath.FromSlash(e.name)), target, e.dir})
	}
	sort.Slice(moves, func(i, j int) bool { return moves[i].to < moves[j].to })
	if err := s.ctx.Err(); err != nil {
		return err
	}
	for _, m := range moves {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		if err := Confined(root, m.to); err != nil {
			return err
		}
		if m.dir {
			if err := os.MkdirAll(m.to, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(m.to), 0700); err != nil {
			return err
		}
		// Link publishes without replacement even on platforms where Rename replaces.
		if err := os.Link(m.from, m.to); err != nil {
			return err
		}
	}
	return nil
}

// Publish combines previously validated containers before modifying a package.
func Publish(ctx context.Context, root, stage, destination string) error {
	if err := Confined(root, stage); err != nil {
		return err
	}
	if err := Confined(root, destination); err != nil {
		return err
	}
	s := staging{ctx: ctx, root: stage, entries: map[string]entry{}}
	err := filepath.WalkDir(stage, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == stage {
			return nil
		}
		if err := Confined(root, p); err != nil {
			return err
		}
		if !d.IsDir() && !d.Type().IsRegular() {
			return fmt.Errorf("invalid staged file")
		}
		rel, err := filepath.Rel(stage, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !SafePath(rel) {
			return fmt.Errorf("invalid staged member")
		}
		s.entries[strings.ToLower(rel)] = entry{rel, d.IsDir()}
		return nil
	})
	if err != nil {
		return err
	}
	return s.publish(root, Request{Destination: destination})
}
