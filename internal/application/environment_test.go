package application

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/relocation"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

func envMap(items []string) map[string]string {
	result := map[string]string{}
	for _, item := range items {
		k, v, ok := strings.Cut(item, "=")
		if ok {
			result[strings.ToUpper(k)] = v
		}
	}
	return result
}
func envFixture(t *testing.T, values map[string]string) environment.Context {
	t.Helper()
	root := testutil.NewRoot(t, "app-env")
	_, paths, err := environment.Load(environment.OSFilesystem{}, root.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.State, 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"schema_version": 1, "values": values})
	if err := os.WriteFile(filepath.Join(paths.State, "environment.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return environment.Context{Identity: root.Identity, Paths: paths}
}

func TestApplicationEnvironmentPreservesCallerAndGit(t *testing.T) {
	local := envFixture(t, map[string]string{"TOOL_HOME": "portable", "PATH": ""})
	foreign := testutil.TempDir(t)
	parent := []string{"SCOOP=" + foreign, "SCOOP_GLOBAL=" + foreign, "SCOOP_CACHE=stale", "PORTSCOOP_REQUEST_FILE=stale", "PORTSCOOP_EXEC_TOKEN=secret", "PATH=" + filepath.Join(foreign, "shims") + `;C:\Windows\System32`, "GIT_CONFIG_GLOBAL=legitimate", "GIT_SSH_COMMAND=application-ssh", "GIT_TERMINAL_PROMPT=1", "KEEP_ME=yes"}
	before := append([]string{}, parent...)
	child, err := BuildEnvironment(local, parent)
	if err != nil {
		t.Fatal(err)
	}
	got := envMap(child)
	if got["TOOL_HOME"] != "portable" || got["KEEP_ME"] != "yes" || got["SCOOP"] != local.Paths.Scoop || got["PORTSCOOP_REQUEST_FILE"] != "" || got["PORTSCOOP_EXEC_TOKEN"] != "" || got["GIT_CONFIG_GLOBAL"] != "legitimate" || got["GIT_SSH_COMMAND"] != "application-ssh" || got["GIT_TERMINAL_PROMPT"] != "1" {
		t.Fatalf("child=%v", got)
	}
	if strings.Contains(got["PATH"], foreign) || !strings.Contains(got["PATH"], local.Paths.Shims) {
		t.Fatalf("PATH=%s", got["PATH"])
	}
	if !reflect.DeepEqual(parent, before) {
		t.Fatal("caller environment mutated")
	}
	backend, err := runner.BuildEnvironment(local, parent, nil)
	if err != nil {
		t.Fatal(err)
	}
	private := envMap(backend)
	if private["GIT_SSH_COMMAND"] != "" || private["GIT_CONFIG_GLOBAL"] == "legitimate" || private["GIT_TERMINAL_PROMPT"] != "0" {
		t.Fatal("backend Git isolation changed")
	}
}

func TestPortableEnvironmentRejectsInvalidState(t *testing.T) {
	for _, values := range []map[string]string{{"SCOOP": "wrong"}, {"GIT_DIR": "wrong"}, {"SystemRoot": "wrong"}, {"Path": `C:\outside`}, {"a": "1", "A": "2"}, {"BAD=KEY": "value"}, {"GOOD": "a\x00b"}} {
		local := envFixture(t, values)
		if _, err := BuildEnvironment(local, nil); err == nil {
			t.Errorf("accepted %v", values)
		}
	}
	local := envFixture(t, map[string]string{})
	if err := os.WriteFile(filepath.Join(local.Paths.State, "environment.json"), []byte(`{"schema_version":1,"values":null}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildEnvironment(local, nil); err == nil {
		t.Fatal("accepted malformed state")
	}
}

func TestRelocatedEnvironmentIsApplied(t *testing.T) {
	local := envFixture(t, map[string]string{})
	path := filepath.Join(local.Paths.State, "environment.json")
	data, _ := json.Marshal(map[string]any{"schema_version": 1, "values": map[string]string{"TOOL_HOME": local.Paths.Apps, "PATH": local.Paths.Shims}})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := (relocation.Manager{}).Capture(local.Paths); err != nil {
		t.Fatal(err)
	}
	old := local.Paths.Root
	moved := old + " moved"
	if err := os.Rename(old, moved); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(moved, old)
	local.Paths, _ = environment.PathsAt(moved)
	if err := (relocation.Manager{}).Prepare(local.Paths); err != nil {
		t.Fatal(err)
	}
	child, err := BuildEnvironment(local, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := envMap(child); got["TOOL_HOME"] != local.Paths.Apps || strings.Contains(got["PATH"], old+string(filepath.Separator)) {
		t.Fatalf("%v", got)
	}
}

func TestApplicationGitUsesCallerConfiguration(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("Git qualification prerequisite is unavailable")
	}
	local := envFixture(t, map[string]string{})
	config := filepath.Join(testutil.TempDir(t), "application.gitconfig")
	if err := os.WriteFile(config, []byte("[portscoop]\nfixture = caller-config\n"), 0600); err != nil {
		t.Fatal(err)
	}
	parent := append(os.Environ(), "GIT_CONFIG_GLOBAL="+config, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_COUNT=0")
	child, err := BuildEnvironment(local, parent)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(git, "config", "--global", "--get", "portscoop.fixture")
	cmd.Env = child
	cmd.Dir = local.Paths.Root
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "caller-config" {
		t.Fatalf("%q %v", out, err)
	}
}
