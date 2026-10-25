package localipc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"

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

func TestTransientSharingViolationDoesNotLoseAcknowledgement(t *testing.T) {
	server, err := New(testutil.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := Open(server.Directory())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Write(ctx, map[string]string{"state": "committed"}); err != nil {
		t.Fatal(err)
	}
	path, _ := windows.UTF16PtrFromString(filepath.Join(server.Directory(), "server-0000.json"))
	held, err := windows.CreateFile(path, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	unlock := func() { once.Do(func() { _ = windows.CloseHandle(held) }) }
	defer unlock()
	go func() { time.Sleep(60 * time.Millisecond); unlock() }()
	var got struct {
		State string `json:"state"`
	}
	if err := client.Read(ctx, &got); err != nil {
		t.Fatal("transient message lock lost acknowledgement:", err)
	}
	if got.State != "committed" {
		t.Fatal(got)
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

func TestPrivateSecurityDescriptorUsesSIDIdentity(t *testing.T) {
	sid, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		t.Fatal(err)
	}
	// SYSTEM is a deterministic alias regression without changing accounts or
	// granting permissions on disk. Administrator SIDs can abbreviate as LA too.
	for _, tc := range []struct {
		name, sddl string
		valid      bool
	}{
		{"abbreviated SID", "O:S-1-5-18D:P(A;OICI;FA;;;S-1-5-18)", true},
		{"extra principal", "O:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;WD)", false},
		{"wrong trustee", "O:SYD:P(A;OICI;FA;;;WD)", false},
		{"wrong owner", "O:BAD:P(A;OICI;FA;;;SY)", false},
		{"unprotected", "O:SYD:(A;OICI;FA;;;SY)", false},
		{"empty ACL", "O:SYD:P", false},
		{"null ACL", "O:SYD:PNO_ACCESS_CONTROL", false},
		{"partial access", "O:SYD:P(A;OICI;FR;;;SY)", false},
		{"no inheritance", "O:SYD:P(A;;FA;;;SY)", false},
		{"inherit only", "O:SYD:P(A;OICIIO;FA;;;SY)", false},
		{"deny entry", "O:SYD:P(D;OICI;FA;;;SY)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(tc.sddl)
			if err != nil {
				t.Fatal(err)
			}
			if tc.valid && !strings.Contains(sd.String(), ";;;SY)") {
				t.Fatal("fixture did not exercise SDDL SID abbreviation:", sd.String())
			}
			if err := validateSecurityDescriptor(sd, sid); (err == nil) != tc.valid {
				t.Fatalf("valid=%t descriptor=%s error=%v", tc.valid, sd.String(), err)
			}
		})
	}
}
