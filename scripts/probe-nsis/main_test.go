package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"
)

func TestListingPreservesDuplicateAndUnknownSizes(t *testing.T) {
	m, err := parseListing("Path = Logos\\icon.png\r\nSize = 4\r\n\r\nPath = Logos\\icon.png\r\nSize = 7\r\n\r\nPath = uninstall.exe\r\nSize = \r\n")
	if err != nil || len(m) != 3 {
		t.Fatalf("%v %v", m, err)
	}
	if *m[0].Size != 4 || *m[1].Size != 7 || m[2].Size != nil || m[0].Path != m[1].Path {
		t.Fatalf("lost inventory evidence: %+v", m)
	}
}

func TestProbeChild(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "probe-sleep" {
		return
	}
	time.Sleep(20 * time.Second)
	os.Exit(0)
}

func TestProcessDeadline(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, _, err = runFor(exe, []string{"-test.run=^TestProbeChild$", "--", "probe-sleep"}, io.Discard, 1024, 100*time.Millisecond)
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("deadline failed: %v", err)
	}
}

func TestListingRejectsInvalidSize(t *testing.T) {
	for _, text := range []string{"", "Size = 1", "Path = a\nSize = -1", "Path = a\nSize = 1\nSize = 2"} {
		if _, err := parseListing(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}

func TestUnknownSizeDuplicateBoundariesAreAmbiguous(t *testing.T) {
	known := int64(3)
	for _, ms := range [][]member{{{Path: "a", Size: nil}, {Path: "a", Size: &known}}, {{Path: "a", Size: &known}, {Path: "a", Size: nil}}} {
		if _, err := boundaries(ms, "a"); err == nil {
			t.Fatal("accepted ambiguous boundaries")
		}
	}
	if sizes, err := boundaries([]member{{Path: "a"}}, "a"); err != nil || len(sizes) != 1 || sizes[0] != -1 {
		t.Fatalf("unknown singleton: %v %v", sizes, err)
	}
	if sizes, err := boundaries([]member{{Path: "a", Size: &known}, {Path: "a", Size: &known}}, "a"); err != nil || len(sizes) != 2 {
		t.Fatalf("known duplicate: %v %v", sizes, err)
	}
}

func TestOutputBoundCancelsWithoutWritingOverflow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var b bytes.Buffer
	w := boundedWriter{w: &b, remaining: 3, cancel: cancel}
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if n, err := w.Write([]byte("d")); n != 0 || err == nil || ctx.Err() == nil || b.String() != "abc" {
		t.Fatalf("limit not enforced: %d %v %q", n, err, b.String())
	}
}
