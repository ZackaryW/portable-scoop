package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

func environmentMap(items []string) map[string]string {
	values := map[string]string{}
	for _, item := range items {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			values[strings.ToUpper(parts[0])] = parts[1]
		}
	}
	return values
}

type fakeEnvironmentProcess struct {
	values  map[string]string
	failure error
}

func (f *fakeEnvironmentProcess) Run(ctx context.Context, command runner.Command) (int, error) {
	f.values = environmentMap(command.Environment)
	return 0, f.failure
}

func TestChildEnvironmentIsExplicitAndDoesNotLeakOnFailure(t *testing.T) {
	root := testutil.NewRoot(t, "work")
	id, paths, err := environment.Load(environment.OSFilesystem{}, root.Path)
	if err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	ordinary := filepath.Join(profile, "scoop")
	custom := filepath.Join(t.TempDir(), "custom-root")
	marker := filepath.Join(custom, "apps", "scoop", "current", "bin", "scoop.ps1")
	if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(t.TempDir(), "foreign")
	parent := []string{"USERPROFILE=" + profile, "SCOOP=" + foreign, "SCOOP_GLOBAL=" + foreign, "SCOOP_CACHE=wrong", "SCOOP_PATH=wrong", "PORTSCOOP_REQUEST_FILE=wrong", "XDG_CONFIG_HOME=wrong", "TEMP=wrong", "TMP=wrong", "KEEP_ME=yes", "PATH=" + strings.Join([]string{filepath.Join(ordinary, "shims"), filepath.Join(ordinary, "apps", "go", "current", "bin"), filepath.Join(custom, "shims"), filepath.Join(foreign, "shims"), `C:\Windows\System32`}, ";"), "PSModulePath=" + filepath.Join(ordinary, "modules") + `;C:\Windows\Modules`}
	unchanged := append([]string(nil), parent...)
	parent = append(parent, "GIT_DIR=C:\\foreign\\repo", "GIT_WORK_TREE=C:\\foreign", "GIT_CONFIG_COUNT=7", "GIT_SSH_COMMAND=foreign-command")
	unchanged = append([]string(nil), parent...)
	callerBefore := os.Getenv("SCOOP")
	for _, failure := range []error{nil, errors.New("fake backend failure")} {
		child, err := runner.BuildEnvironment(environment.Context{Identity: id, Paths: paths}, parent, nil)
		if err != nil {
			t.Fatal(err)
		}
		fake := &fakeEnvironmentProcess{failure: failure}
		fake.Run(context.Background(), runner.Command{Environment: child})
		if fake.values["SCOOP"] != paths.Scoop || fake.values["SCOOP_CACHE"] != paths.Cache || fake.values["SCOOP_GLOBAL"] != paths.Global || fake.values["TEMP"] != paths.Temp || fake.values["KEEP_ME"] != "yes" {
			t.Fatalf("portable context was not explicit: %+v", fake.values)
		}
		if fake.values["PATH"] != paths.Shims+`;C:\Windows\System32` || fake.values["PSMODULEPATH"] != `C:\Windows\Modules` {
			t.Fatal("other Scoop activation paths leaked into child")
		}
		if _, exists := fake.values["PORTSCOOP_REQUEST_FILE"]; exists {
			t.Fatal("inherited request transport leaked")
		}
		if fake.values["GIT_DIR"] != "" || fake.values["GIT_WORK_TREE"] != "" || fake.values["GIT_SSH_COMMAND"] != "" || fake.values["GIT_CONFIG_COUNT"] != "2" || fake.values["GIT_CONFIG_GLOBAL"] != filepath.Join(paths.State, "git-global.config") {
			t.Fatal("inherited Git controls escaped bucket isolation")
		}
	}
	if !reflect.DeepEqual(parent, unchanged) || os.Getenv("SCOOP") != callerBefore {
		t.Fatal("child environment construction changed the caller")
	}
}

func TestContextCannotRedirectPrivatePaths(t *testing.T) {
	paths, _ := environment.PathsAt(t.TempDir())
	paths.Cache = t.TempDir()
	if _, err := runner.BuildEnvironment(environment.Context{Paths: paths}, nil, nil); err == nil {
		t.Fatal("accepted a redirected cache path")
	}
}

func TestManagedGitNeverSearchesCallerToolsOrConfig(t *testing.T) {
	paths, _ := environment.PathsAt(t.TempDir())
	parent := []string{`SystemRoot=C:\Windows`, `PATH=C:\host-tools`, `HOME=C:\user`, `GIT_CONFIG_SYSTEM=C:\host.config`, `GIT_CONFIG_GLOBAL=C:\user.config`, `GIT_CONFIG_COUNT=1`, `GIT_CONFIG_KEY_0=credential.helper`, `GIT_CONFIG_VALUE_0=host-helper`, `GIT_EXEC_PATH=C:\host-tools`, `GIT_DIR=C:\other\.git`}
	before := append([]string{}, parent...)
	env, err := runner.GitEnvironment(environment.Context{Paths: paths}, parent)
	if err != nil {
		t.Fatal(err)
	}
	values := environmentMap(env)
	if values["GIT_CONFIG_SYSTEM"] != "NUL" || values["GIT_CONFIG_GLOBAL"] != "NUL" || values["GIT_CONFIG_NOSYSTEM"] != "1" || values["GIT_DIR"] != "" {
		t.Fatal("inherited Git config")
	}
	if strings.Contains(values["PATH"], "host-tools") || !strings.HasPrefix(values["GIT_EXEC_PATH"], paths.Control) || !strings.HasPrefix(values["HOME"], paths.State) {
		t.Fatal("host tool discovery retained")
	}
	if !reflect.DeepEqual(parent, before) {
		t.Fatal("caller environment changed")
	}
}

func TestPowerShellResolutionIgnoresPATH(t *testing.T) {
	systemRoot := t.TempDir()
	exe := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, err := runner.PowerShell([]string{"SystemRoot=" + systemRoot, "PATH=unrelated"}, nil)
	if err != nil || resolved != exe {
		t.Fatalf("PowerShell lookup did not use explicit Windows path: %s %v", resolved, err)
	}
	if _, err := runner.PowerShell([]string{"PATH=" + filepath.Dir(exe)}, nil); err == nil {
		t.Fatal("PowerShell fell back to PATH")
	}
}
