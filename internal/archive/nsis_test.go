package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portscoop/internal/runner"
)

func nsisRecord(name, size string) string {
	return "Path = " + name + "\nSize = " + size + "\nAttributes = \n\n"
}
func nsisPayload() []byte { return []byte("MZtest-stub\xef\xbe\xad\xdeNullsoftInsttest") }

type fakeNSIS struct {
	listing string
	bodies  map[string]string
	fail    string
	calls   int
}

func (f *fakeNSIS) List(context.Context, string) ([]byte, error) { return []byte(f.listing), nil }
func (f *fakeNSIS) Stream(_ context.Context, _ string, name string, w io.Writer, _ int64) error {
	f.calls++
	_, err := io.Copy(w, strings.NewReader(f.bodies[name]))
	if err != nil {
		return err
	}
	if name == f.fail {
		return fmt.Errorf("native read failed")
	}
	return nil
}

func TestNSISRetainsLastExactEntryAndBoundedUnknown(t *testing.T) {
	f := &fakeNSIS{listing: nsisRecord(`$0\data`, "0") + nsisRecord(`$0\data`, "3") + nsisRecord(`$0\data`, "4") + nsisRecord("uninstall.exe", ""), bodies: map[string]string{`$0\data`: "oldLAST", "uninstall.exe": "inert"}}
	root, q := setup(t, nsisPayload(), "7z")
	q.NSIS = func() (NSISReader, error) { return f, nil }
	if err := Extract(context.Background(), root, q); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{filepath.Join("$0", "data"): "LAST", "uninstall.exe": "inert"} {
		got, err := os.ReadFile(filepath.Join(q.Destination, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q %v", name, got, err)
		}
	}
}

func TestNSISInventoryRejections(t *testing.T) {
	for name, listing := range map[string]string{
		"response-file": nsisRecord("@names.txt", "0"),
		"traversal":     nsisRecord(`..\escape`, "1"), "ads": nsisRecord("file:stream", "1"), "device": nsisRecord("NUL.txt", "1"),
		"control": nsisRecord("bad\x01name", "1"), "case": nsisRecord("Foo/a", "1") + nsisRecord("foo/b", "1"),
		"type": nsisRecord("a", "1") + nsisRecord("a/b", "1"), "alias": nsisRecord(`a\b`, "1") + nsisRecord("a/b", "1"),
		"unknown-first": nsisRecord("a", "") + nsisRecord("a", "1"), "unknown-last": nsisRecord("a", "1") + nsisRecord("a", ""),
		"negative": nsisRecord("a", "-1"), "overflow": nsisRecord("a", "99999999999999999999"), "extra-field": nsisRecord("a", "1") + "Symbolic Link = x\n",
		"reparse":        strings.Replace(nsisRecord("a", "1"), "Attributes = ", "Attributes = L", 1),
		"repeated-field": "Path = a\nSize = 1\nSize = 1\nAttributes = \n",
		"missing-size":   "Path = a\nAttributes = \n", "newline-name": nsisRecord("a\ninjection", "1"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseNSIS([]byte(listing), DefaultLimits); err == nil {
				t.Fatal("accepted ambiguous inventory")
			}
		})
	}
}

func TestNSISFailurePublishesNothing(t *testing.T) {
	for _, mode := range []string{"short", "long", "exit", "unknown-limit", "discarded-limit", "count", "late-unknown", "late-exit", "invalid-inventory"} {
		t.Run(mode, func(t *testing.T) {
			f := &fakeNSIS{listing: nsisRecord("first", "2") + nsisRecord("second", "2"), bodies: map[string]string{"first": "ok", "second": "ok"}}
			root, q := setup(t, nsisPayload(), "7z")
			switch mode {
			case "short":
				f.bodies["second"] = "x"
			case "long":
				f.bodies["second"] = "long"
			case "exit", "late-exit":
				f.fail = "second"
			case "unknown-limit":
				f.listing = nsisRecord("second", "")
				f.bodies["second"] = "too big"
				q.Limits = Limits{10, 20, 2}
			case "discarded-limit":
				f.listing = nsisRecord("second", "2") + nsisRecord("second", "2")
				q.Limits = Limits{10, 3, 2}
			case "count":
				q.Limits = Limits{1, 10, 10}
			case "late-unknown":
				f.listing = nsisRecord("first", "2") + nsisRecord("second", "")
				q.Limits = Limits{10, 3, 2}
			case "invalid-inventory":
				f.listing += nsisRecord("../escape", "0")
			}
			q.NSIS = func() (NSISReader, error) { return f, nil }
			if err := Extract(context.Background(), root, q); err == nil {
				t.Fatal("accepted failed extraction")
			}
			entries, _ := os.ReadDir(q.Destination)
			if len(entries) != 0 {
				t.Fatal("published failed archive", entries)
			}
			if mode == "invalid-inventory" && f.calls != 0 {
				t.Fatal("streamed before complete inventory validation")
			}
		})
	}
}

func TestNSISNeverFallbackForOtherFormats(t *testing.T) {
	for _, format := range []string{"zip", "7z", "tar", "tar.gz", "gzip", "file", "nsis"} {
		t.Run(format, func(t *testing.T) {
			root, q := setup(t, []byte("MZwrong executable"), format)
			q.NSIS = func() (NSISReader, error) { t.Fatal("native fallback for unrecognized content"); return nil, nil }
			if err := Extract(context.Background(), root, q); err == nil {
				t.Fatal("accepted wrong format")
			}
		})
	}
	root, q := setup(t, zipFixture(t, fixtureMember{"$literal/a", "ok", 0600}), "7z")
	q.NSIS = func() (NSISReader, error) { t.Fatal("ZIP used native helper"); return nil, nil }
	if err := Extract(context.Background(), root, q); err != nil {
		t.Fatal(err)
	}
}

type processFunc func(context.Context, runner.Command) (int, error)

func (f processFunc) Run(ctx context.Context, c runner.Command) (int, error) { return f(ctx, c) }
func TestNSISProcessLimitsAndSelection(t *testing.T) {
	for _, mode := range []string{"success", "output", "stderr", "write", "exit", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			n := NativeNSIS{Executable: filepath.Join(t.TempDir(), "7z.exe"), Runner: processFunc(func(ctx context.Context, c runner.Command) (int, error) {
				if c.MemoryLimit != nsisMemoryLimit {
					t.Fatal("missing job memory cap")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				joined := strings.Join(c.Arguments, "|")
				if !strings.Contains(joined, "-spd|-ssc|-tNsis") || !strings.HasSuffix(joined, `--|payload|$0\name`) {
					t.Fatal(joined)
				}
				switch mode {
				case "output":
					_, _ = c.Streams.Out.Write([]byte("12345"))
					if ctx.Err() == nil {
						t.Fatal("overflow did not cancel")
					}
				case "stderr":
					_, _ = c.Streams.Err.Write(make([]byte, nsisDiagnosticLimit+1))
					if ctx.Err() == nil {
						t.Fatal("diagnostic overflow did not cancel")
					}
				case "exit":
					return 2, nil
				case "cancel":
					return 0, context.Canceled
				default:
					_, _ = c.Streams.Out.Write([]byte("data"))
				}
				return 0, nil
			})}
			var b bytes.Buffer
			var out io.Writer = &b
			if mode == "write" {
				out = failWriter{}
			}
			err := n.Stream(context.Background(), "payload", `$0\name`, out, 4)
			if (err == nil) != (mode == "success") {
				t.Fatalf("result %v", err)
			}
		})
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("disk failure") }

// Opt-in real fixture runs only inside the disposable VM. Every executable and
// payload is pinned before the production process adapter is used.
func TestNativeNSISQualification(t *testing.T) {
	data, native := nativeNSISFixture(t)
	root, q := setup(t, data, "7z")
	q.NSIS = func() (NSISReader, error) { return native, nil }
	if err := Extract(context.Background(), root, q); err != nil {
		t.Fatal(err)
	}
	for path, size := range map[string]int64{"$0/spltmp.bmp": 259424, "Logos/PotPlayer.png": 8770, "uninstall.exe": 241929} {
		info, err := os.Stat(filepath.Join(q.Destination, filepath.FromSlash(path)))
		if err != nil || info.Size() != size {
			t.Fatalf("member %s: %v %v", path, info, err)
		}
	}
	if _, err := os.Stat(filepath.Join(q.Destination, "PotPlayerMini64.exe")); err != nil {
		t.Fatal(err)
	}
	t.Log("complete pinned NSIS payload extracted; duplicate winners and unknown-size member verified")
}

func nativeNSISFixture(t *testing.T) ([]byte, NativeNSIS) {
	t.Helper()
	fixture := os.Getenv("PORTSCOOP_NSIS_FIXTURE")
	if fixture == "" {
		t.Skip("requires disposable VM fixture")
	}
	for path, want := range map[string]string{
		"potplayer.exe":   "28890593ea6b1156ca93cd79622fce4e8c1740bfdbe6ee907e3190ec7c3f51d3",
		"sevenzip/7z.exe": "94205cfc054dd3bbeb978905bc619a65b3f04e9e73bb2717841110b6f46d962e",
		"sevenzip/7z.dll": "78bf6b668d75801e3aa180b4ccb16e6d8395fba80dec8a065055bb6f8dfabb82",
	} {
		data, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(path)))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != want {
			t.Fatalf("fixture integrity %s: %v", path, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(fixture, "potplayer.exe"))
	if err != nil {
		t.Fatal(err)
	}
	native := NativeNSIS{Executable: filepath.Join(fixture, "sevenzip", "7z.exe"), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}}
	return data, native
}

func TestNativeNSISFailureQualification(t *testing.T) {
	data, native := nativeNSISFixture(t)
	root, q := setup(t, data[:1<<20], "7z")
	q.NSIS = func() (NSISReader, error) { return native, nil }
	if err := Extract(context.Background(), root, q); err == nil {
		t.Fatal("corrupt NSIS accepted")
	}
	entries, _ := os.ReadDir(q.Destination)
	if len(entries) != 0 {
		t.Fatal("corrupt NSIS published files")
	}
	if err := os.WriteFile(q.Source, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := native.Stream(ctx, q.Source, "uninstall.exe", io.Discard, DefaultLimits.MemberBytes); err != context.DeadlineExceeded {
		t.Fatalf("timeout result: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("cancelled extraction did not stop promptly")
	}
	t.Log("corrupt payload publishes nothing; native decompression deadline stops the process")
}
