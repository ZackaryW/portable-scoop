// Command probe-nsis records extraction-only behavior of a pinned native tool.
// It is a qualification probe, not a package installation or extraction policy.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type member struct {
	Path string `json:"path"`
	Size *int64 `json:"size"`
}

type streamResult struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
	Error  string `json:"error,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

type report struct {
	PayloadSHA256 string         `json:"payload_sha256"`
	ToolSHA256    string         `json:"tool_sha256"`
	LibrarySHA256 string         `json:"library_sha256"`
	ListingError  string         `json:"listing_error,omitempty"`
	ListingStderr string         `json:"listing_stderr,omitempty"`
	Members       []member       `json:"members"`
	Streams       []streamResult `json:"streams"`
}

func digest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// boundedWriter cancels the process on excessive output rather than continuing
// to drain an unbounded stream or buffering an entire extracted member.
type boundedWriter struct {
	w         io.Writer
	remaining int64
	n         int64
	cancel    context.CancelFunc
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		w.cancel()
		return 0, fmt.Errorf("probe output limit exceeded")
	}
	n, err := w.w.Write(p)
	w.n += int64(n)
	w.remaining -= int64(n)
	return n, err
}

func run(tool string, args []string, output io.Writer, limit int64) (int64, string, error) {
	return runFor(tool, args, output, limit, 30*time.Second)
}

func runFor(tool string, args []string, output io.Writer, limit int64, timeout time.Duration) (int64, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var diagnostic bytes.Buffer
	out := &boundedWriter{w: output, remaining: limit, cancel: cancel}
	errout := &boundedWriter{w: &diagnostic, remaining: 64 << 10, cancel: cancel}
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = filepath.Dir(tool)
	cmd.Stdout, cmd.Stderr = out, errout
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	return out.n, diagnostic.String(), err
}

func parseListing(text string) ([]member, error) {
	var result []member
	var current *member
	seenSize := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		key, value, found := strings.Cut(line, " = ")
		if !found {
			continue
		}
		switch key {
		case "Path":
			result = append(result, member{Path: value})
			current = &result[len(result)-1]
			seenSize = false
		case "Size":
			if current == nil || seenSize {
				return nil, fmt.Errorf("unexpected Size record")
			}
			seenSize = true
			if value != "" {
				n, err := strconv.ParseInt(value, 10, 64)
				if err != nil || n < 0 {
					return nil, fmt.Errorf("invalid Size")
				}
				current.Size = &n
			}
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no listed members")
	}
	return result, nil
}

func boundaries(members []member, name string) ([]int64, error) {
	var sizes []int64
	unknown := false
	for _, m := range members {
		if m.Path == name {
			if m.Size == nil {
				unknown = true
				sizes = append(sizes, -1)
			} else {
				sizes = append(sizes, *m.Size)
			}
		}
	}
	if len(sizes) > 1 && unknown {
		return nil, fmt.Errorf("ambiguous duplicate boundaries")
	}
	return sizes, nil
}

func main() {
	tool := flag.String("tool", "", "absolute path of verified full 7-Zip executable")
	payload := flag.String("payload", "", "NSIS payload; never executed")
	want := flag.String("sha256", "", "required payload SHA-256")
	wantTool := flag.String("tool-sha256", "", "required executable SHA-256")
	wantLibrary := flag.String("library-sha256", "", "required adjacent 7z.dll SHA-256")
	output := flag.String("report", "", "new qualification JSON report")
	flag.Parse()
	fail := func(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	if !filepath.IsAbs(*tool) || !filepath.IsAbs(*payload) || !filepath.IsAbs(*output) || flag.NArg() != 0 {
		fail(fmt.Errorf("absolute tool, payload, and report paths required"))
	}
	r := report{}
	for _, item := range []struct {
		path, want string
		value      *string
	}{
		{*payload, *want, &r.PayloadSHA256}, {*tool, *wantTool, &r.ToolSHA256},
		{filepath.Join(filepath.Dir(*tool), "7z.dll"), *wantLibrary, &r.LibrarySHA256},
	} {
		got, err := digest(item.path)
		if err != nil {
			fail(err)
		}
		if len(item.want) != 64 || !strings.EqualFold(got, item.want) {
			fail(fmt.Errorf("digest mismatch: %s", item.path))
		}
		*item.value = got
	}
	var listing bytes.Buffer
	var err error
	_, r.ListingStderr, err = run(*tool, []string{"l", "-slt", "-ba", "-tNsis", "-sccUTF-8", *payload}, &listing, 8<<20)
	if err != nil {
		r.ListingError = err.Error()
	} else {
		r.Members, err = parseListing(listing.String())
		if err != nil {
			r.ListingError = err.Error()
		}
	}
	// Probe only repeated or unknown-size paths, once each. The result preserves
	// their ordered listing records; it does not select a duplicate winner.
	counts := map[string]int{}
	for _, m := range r.Members {
		counts[m.Path]++
	}
	seen := map[string]bool{}
	for _, m := range r.Members {
		if seen[m.Path] || (counts[m.Path] < 2 && m.Size != nil) {
			continue
		}
		seen[m.Path] = true
		if _, err := boundaries(r.Members, m.Path); err != nil {
			r.Streams = append(r.Streams, streamResult{Path: m.Path, Error: err.Error()})
			continue
		}
		h := sha256.New()
		n, diagnostic, err := run(*tool, []string{"x", "-so", "-spd", "-ssc", "-tNsis", "-bso0", "-bsp0", "-bse2", "--", *payload, m.Path}, h, 128<<20)
		s := streamResult{Path: m.Path, Bytes: n, SHA256: hex.EncodeToString(h.Sum(nil)), Stderr: diagnostic}
		if err != nil {
			s.Error = err.Error()
		}
		r.Streams = append(r.Streams, s)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fail(err)
	}
	f, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fail(err)
	}
	_, err = f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err != nil {
		fail(err)
	}
	if closeErr != nil {
		fail(closeErr)
	}
	if r.ListingError != "" {
		fail(fmt.Errorf("listing failed: %s", r.ListingError))
	}
	for _, s := range r.Streams {
		if s.Error != "" {
			fail(fmt.Errorf("stream failed for %s: %s", s.Path, s.Error))
		}
	}
	fmt.Printf("Recorded %d members and %d exceptional-path streams\n", len(r.Members), len(r.Streams))
}
