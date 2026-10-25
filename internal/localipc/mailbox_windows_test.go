package localipc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portscoop/internal/testutil"
)

func TestPrivateControlRoundTripAndCleanup(t *testing.T) {
	s, err := New(testutil.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := Open(s.Directory())
	if err != nil {
		t.Fatal(err)
	}
	type message struct {
		Args []string `json:"args"`
	}
	want := message{[]string{"", "雪", `a"b`, "%!&<>"}}
	if err := s.Write(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	var got message
	if err := c.Read(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Args, "\x00") != strings.Join(want.Args, "\x00") {
		t.Fatal(got)
	}
	if err := c.Write(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(context.Background(), &got); err == nil || !strings.Contains(err.Error(), "disconnected") {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Directory()); !os.IsNotExist(err) {
		t.Fatal("private transport retained", err)
	}
}

func TestControlBoundsCancellationAndMalformedData(t *testing.T) {
	s, err := New(testutil.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := Open(s.Directory())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Write(context.Background(), strings.Repeat("x", MaxMessage)); err == nil {
		t.Fatal("oversize message accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	var value struct {
		Value string `json:"value"`
	}
	if err := c.Read(ctx, &value); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Directory(), "server-0000.json"), []byte(`{"unknown":"field"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.Read(context.Background(), &value); err == nil {
		t.Fatal("unknown message fields accepted")
	}
}

func TestOrdinaryDirectoryIsNotAControlEndpoint(t *testing.T) {
	parent := testutil.TempDir(t)
	dir := filepath.Join(parent, "portscoop-execution-"+strings.Repeat("a", 64))
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("inherited DACL accepted as private")
	}
}
