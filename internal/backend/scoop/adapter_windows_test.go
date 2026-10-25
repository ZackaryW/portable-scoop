package scoop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"portscoop/internal/testutil"
)

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
