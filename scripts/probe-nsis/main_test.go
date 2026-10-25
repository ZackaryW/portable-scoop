package main

import (
	"bytes"
	"context"
	"testing"
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

func TestListingRejectsInvalidSize(t *testing.T) {
	for _, text := range []string{"", "Size = 1", "Path = a\nSize = -1", "Path = a\nSize = 1\nSize = 2"} {
		if _, err := parseListing(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
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
