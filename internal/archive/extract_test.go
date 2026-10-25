package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureMember struct {
	name, body string
	mode       fs.FileMode
}

func zipFixture(t *testing.T, ms ...fixtureMember) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, m := range ms {
		h := &zip.FileHeader{Name: m.name, Method: zip.Deflate}
		h.SetMode(m.mode)
		w, err := z.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(m.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func tarFixture(t *testing.T, link bool) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	h := &tar.Header{Name: "./folder/file.txt", Mode: 0600, Size: 5}
	if link {
		h.Typeflag = tar.TypeSymlink
		h.Linkname = "../../outside"
		h.Size = 0
	}
	if err := w.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	if !link {
		w.Write([]byte("hello"))
	}
	w.Close()
	return b.Bytes()
}
func gz(t *testing.T, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	g := gzip.NewWriter(&b)
	g.Write(data)
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func setup(t *testing.T, data []byte, format string) (string, Request) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "input."+format)
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "out")
	os.Mkdir(out, 0700)
	return root, Request{Source: source, Destination: out, Format: format}
}
func TestArchiveFamiliesAndSelectors(t *testing.T) {
	for _, format := range []string{"zip", "nupkg", "tar", "tar.gz", "gzip", "7z"} {
		t.Run(format, func(t *testing.T) {
			var data []byte
			want := "folder/file.txt"
			switch format {
			case "zip", "nupkg":
				data = zipFixture(t, fixtureMember{"folder/file.txt", "hello", 0600})
			case "tar":
				data = tarFixture(t, false)
			case "tar.gz":
				data = gz(t, tarFixture(t, false))
			case "gzip":
				data = gz(t, []byte("hello"))
				want = "input"
			case "7z":
				var err error
				data, err = os.ReadFile("testdata/lzma2.7z")
				if err != nil {
					t.Fatal(err)
				}
				want = ""
			}
			root, q := setup(t, data, format)
			if format == "nupkg" {
				q.Format = "zip"
			}
			if err := Extract(context.Background(), root, q); err != nil {
				t.Fatal(err)
			}
			if want != "" {
				got, err := os.ReadFile(filepath.Join(q.Destination, filepath.FromSlash(want)))
				if err != nil || string(got) != "hello" {
					t.Fatal(string(got), err)
				}
			} else {
				count := 0
				filepath.WalkDir(q.Destination, func(p string, d fs.DirEntry, e error) error {
					if e == nil && !d.IsDir() {
						count++
					}
					return e
				})
				if count < 1 {
					t.Fatal("7z fixture empty")
				}
			}
		})
	}
	root, q := setup(t, zipFixture(t, fixtureMember{"prefix/a.txt", "hello", 0600}, fixtureMember{"other.txt", "ignore", 0600}), "zip")
	q.ExtractDir = "prefix"
	q.ExtractTo = "tools"
	if err := Extract(context.Background(), root, q); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(q.Destination, "tools", "a.txt")); err != nil || string(b) != "hello" {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(q.Destination, "other.txt")); !os.IsNotExist(err) {
		t.Fatal("selector ignored")
	}
}
func TestRejectUnsafeArchivesBeforePublication(t *testing.T) {
	for name, data := range map[string][]byte{
		"traversal": zipFixture(t, fixtureMember{"good", "safe", 0600}, fixtureMember{"../outside", "bad", 0600}),
		"case":      zipFixture(t, fixtureMember{"Dir/a", "a", 0600}, fixtureMember{"dir/b", "b", 0600}),
		"duplicate": zipFixture(t, fixtureMember{"a", "a", 0600}, fixtureMember{"a", "b", 0600}),
		"link":      zipFixture(t, fixtureMember{"link", "../outside", fs.ModeSymlink | 0777}),
		"device":    zipFixture(t, fixtureMember{"NUL.txt", "a", 0600}),
		"stream":    zipFixture(t, fixtureMember{"a:stream", "a", 0600}),
		"absolute":  zipFixture(t, fixtureMember{"/absolute", "a", 0600}),
		"malformed": []byte("MZ this is not a ZIP file"),
	} {
		t.Run(name, func(t *testing.T) {
			root, q := setup(t, data, "zip")
			if err := Extract(context.Background(), root, q); err == nil {
				t.Fatal("accepted")
			}
			entries, _ := os.ReadDir(q.Destination)
			if len(entries) != 0 {
				t.Fatal("partially published")
			}
			stages, _ := filepath.Glob(filepath.Join(root, ".portscoop-extract-*"))
			if len(stages) != 0 {
				t.Fatal("staging leaked")
			}
		})
	}
	root, q := setup(t, tarFixture(t, true), "tar")
	if err := Extract(context.Background(), root, q); err == nil {
		t.Fatal("tar link accepted")
	}
	root, q = setup(t, []byte("MZ NSIS installer is not a 7z container"), "7z")
	if err := Extract(context.Background(), root, q); err == nil {
		t.Fatal("renamed EXE accepted")
	}
}
func TestLimitsCancellationAndExistingTargets(t *testing.T) {
	data := zipFixture(t, fixtureMember{"a", "12345", 0600}, fixtureMember{"b", "12345", 0600})
	for _, limit := range []Limits{{1, 100, 100}, {10, 9, 100}, {10, 100, 4}} {
		root, q := setup(t, data, "zip")
		q.Limits = limit
		if err := Extract(context.Background(), root, q); err == nil {
			t.Fatal("limit ignored")
		}
	}
	root, q := setup(t, data, "zip")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Extract(ctx, root, q); err == nil {
		t.Fatal("cancellation ignored")
	}
	os.WriteFile(filepath.Join(q.Destination, "b"), []byte("sentinel"), 0600)
	if err := Extract(context.Background(), root, q); err == nil {
		t.Fatal("overwrote existing")
	}
	if _, err := os.Stat(filepath.Join(q.Destination, "a")); !os.IsNotExist(err) {
		t.Fatal("published before collision check")
	}
	for _, sel := range []string{"../x", `C:\host`, "NUL", "missing"} {
		q.ExtractDir = sel
		if err := Extract(context.Background(), root, q); err == nil {
			t.Fatal("invalid selector", sel)
		}
	}
	for _, p := range []string{"../outside", "a/../../outside"} {
		if err := Confined(root, filepath.Join(root, p)); err == nil {
			t.Fatal("escaped root")
		}
	}
	b, _ := os.ReadFile(filepath.Join(q.Destination, "b"))
	if !strings.EqualFold(string(b), "sentinel") {
		t.Fatal("changed sentinel")
	}
}
