package runner

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"portscoop/internal/environment"
)

//go:embed request-reader.ps1
var requestReader string

func RequestReader() string { return requestReader }

type Request struct {
	SchemaVersion int             `json:"schema_version"`
	ExpectedID    string          `json:"expected_id"`
	Arguments     []string        `json:"arguments"`
	Manifests     json.RawMessage `json:"manifests,omitempty"`
}

// PreparePowerShell passes only a fixed script path on the native command line.
// User arguments live in an invocation-private JSON file and are never code.
func PreparePowerShell(local environment.Context, args, parent []string, streams Streams) (Command, func(), error) {
	return PrepareRequest(local, Request{Arguments: args}, parent, streams)
}

func PrepareRequest(local environment.Context, request Request, parent []string, streams Streams) (Command, func(), error) {
	noop := func() {}
	if err := local.Identity.Validate(); err != nil {
		return Command{}, noop, err
	}
	for _, arg := range request.Arguments {
		if strings.ContainsRune(arg, 0) {
			return Command{}, noop, fmt.Errorf("argument contains NUL")
		}
	}
	child, err := BuildEnvironment(local, parent, nil)
	if err != nil {
		return Command{}, noop, err
	}
	powerShell := local.PowerShell
	if powerShell == "" {
		powerShell, err = PowerShell(parent, nil)
		if err != nil {
			return Command{}, noop, err
		}
	}
	if err := os.MkdirAll(local.Paths.Temp, 0700); err != nil {
		return Command{}, noop, err
	}
	for _, directory := range []string{"git-hooks", "git-templates"} {
		path := filepath.Join(local.Paths.State, directory)
		if err := os.MkdirAll(path, 0700); err != nil {
			return Command{}, noop, err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return Command{}, noop, fmt.Errorf("private Git integration directory is not owned: %s", directory)
		}
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) > 0 {
			return Command{}, noop, fmt.Errorf("private Git hooks/templates must remain empty: %s", directory)
		}
	}
	gitConfig := filepath.Join(local.Paths.State, "git-global.config")
	configFile, configErr := os.OpenFile(gitConfig, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if configErr == nil {
		configErr = configFile.Close()
	} else if os.IsExist(configErr) {
		info, err := os.Lstat(gitConfig)
		if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
			return Command{}, noop, fmt.Errorf("private Git global config must remain an empty regular file")
		}
		configErr = nil
	}
	if configErr != nil {
		return Command{}, noop, configErr
	}
	file, err := os.CreateTemp(local.Paths.Temp, "portscoop-request-*.json")
	if err != nil {
		return Command{}, noop, err
	}
	cleanup := func() { os.Remove(file.Name()) }
	request.SchemaVersion, request.ExpectedID = 1, local.Identity.ID
	request.Arguments = append([]string{}, request.Arguments...)
	data, err := json.Marshal(request)
	if err == nil {
		_, err = file.Write(data)
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return Command{}, noop, err
	}
	child = append(child, "PORTSCOOP_REQUEST_FILE="+file.Name())
	return Command{Executable: powerShell, Arguments: []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", local.Paths.Adapter}, Environment: child, Directory: local.WorkingDirectory, Streams: streams}, cleanup, nil
}
