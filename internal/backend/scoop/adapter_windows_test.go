package scoop

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"portscoop/internal/testutil"
)

func TestPreparedFrozenDownloads(t *testing.T) {
	backend := os.Getenv("PORTSCOOP_TEST_BACKEND_PAYLOAD")
	if backend == "" {
		t.Skip("requires prepared backend")
	}
	payload := []byte("inert controlled EXE contents; never executable")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/download?name=wrong.exe" {
			t.Errorf("unexpected URL %s", r.URL)
		}
		w.Write(payload)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	script, _ := filepath.Abs("../../../tests/backend/download-tests.ps1")
	ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	out, err := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "-Root", testutil.TempDir(t), "-Backend", backend, "-Address", server.URL, "-Digest", fmt.Sprintf("%x", sha256.Sum256(payload))).CombinedOutput()
	if err != nil {
		t.Fatalf("download checks: %v\n%s", err, out)
	}
}

func TestPortableActionExecutor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	script, err := filepath.Abs("../../../tests/backend/compatibility-tests.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	output, err := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "-Root", testutil.TempDir(t)).CombinedOutput()
	if err != nil {
		t.Fatalf("portable action test: %v\n%s", err, output)
	}
}

func TestCheckedFilesystemHelpers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	script, err := filepath.Abs("../../../tests/backend/filesystem-tests.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	out, err := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "-Root", testutil.TempDir(t)).CombinedOutput()
	if err != nil {
		t.Fatalf("filesystem checks: %v\n%s", err, out)
	}
}
