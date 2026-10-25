package scoop

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/relocation"
	"portscoop/internal/runner"
	"portscoop/internal/runtimeinstall"
)

// MainRevision is the initial checkout only. Explicit bucket updates remain
// independent of runtime updates and are never reset by repeated bootstrap.
const MainRevision = "990163c94e827a8a3966f643e86a4861aa13666e"
const MainRepository = "https://github.com/ScoopInstaller/Main"
const bootstrapReceiptPath = ".portscoop/state/bootstrap.json"

type bootstrapReceipt struct {
	Schema     int    `json:"schema_version"`
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Stage      string `json:"stage,omitempty"`
	Complete   bool   `json:"complete"`
}

type Bootstrapper struct{ Runner runner.ProcessRunner }

// Bootstrap locks a ready Store; initialization uses ensureMain inside its own
// unpublished stage. Both paths verify the same helper inventory.
func (b Bootstrapper) Bootstrap(ctx context.Context, local environment.Context, out io.Writer) error {
	m := relocation.Manager{}
	_, err := (relocation.Coordinator{Prepare: m.Prepare, Capture: m.Capture, Operation: "bootstrap"}).Do(ctx, local, func() (int, error) {
		if err := b.ensureMain(ctx, local.Paths); err != nil {
			return 0, err
		}
		return 0, ReportBootstrap(local.Paths, out)
	})
	return err
}

func ReportBootstrap(paths environment.Paths, out io.Writer) error {
	backend, err := Verify(paths)
	if err != nil {
		return err
	}
	inventory, err := verifyToolInventory(paths.Root, backend)
	if err != nil {
		return err
	}
	if out != nil {
		for _, tool := range inventory.Tools {
			fmt.Fprintf(out, "Managed %s %s\n", tool.Name, tool.Version)
		}
		fmt.Fprintln(out, "Main ready at", filepath.Join(paths.Buckets, "main"))
	}
	return nil
}

// boundedGitOutput avoids accumulating unbounded diagnostics from a child.
type boundedGitOutput struct {
	bytes.Buffer
	cancel   context.CancelFunc
	exceeded bool
}

func (w *boundedGitOutput) Write(p []byte) (int, error) {
	if w.Len()+len(p) > 1<<20 {
		w.exceeded = true
		w.cancel()
		return 0, fmt.Errorf("Git output exceeds limit")
	}
	return w.Buffer.Write(p)
}

func (b Bootstrapper) git(ctx context.Context, paths environment.Paths, executable, directory string, args ...string) (string, error) {
	env, err := runner.GitEnvironment(environment.Context{Paths: paths}, os.Environ())
	if err != nil {
		return "", err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	out := &boundedGitOutput{cancel: cancel}
	diagnostic := &boundedGitOutput{cancel: cancel}
	arguments := []string{"-c", "credential.helper=", "-c", "core.hooksPath=NUL", "-c", "init.templateDir=", "-c", "core.fsmonitor=false", "-c", "protocol.ext.allow=never", "-C", directory}
	process := b.Runner
	if process == nil {
		process = runner.Native{}
	}
	code, err := process.Run(bounded, runner.Command{Executable: executable, Arguments: append(arguments, args...), Directory: paths.Root, Environment: env, Streams: runner.Streams{Out: out, Err: diagnostic}})
	if err != nil {
		return "", err
	}
	if code != 0 || out.exceeded || diagnostic.exceeded {
		return "", fmt.Errorf("managed Git failed (%d): %s", code, strings.TrimSpace(diagnostic.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func officialMain(url string) bool {
	return strings.EqualFold(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git"), MainRepository)
}

func (b Bootstrapper) ensureMain(ctx context.Context, paths environment.Paths) error {
	git, err := Helper(paths, "git")
	if err != nil {
		return err
	}
	main, err := runtimeinstall.CheckPath(paths.Root, "scoop/buckets/main")
	if err != nil {
		return err
	}
	var receipt bootstrapReceipt
	err = runtimeinstall.Read(paths.Root, bootstrapReceiptPath, &receipt)
	if err == nil {
		if receipt.Schema != 1 || receipt.Repository != MainRepository || len(receipt.Revision) != 40 || (receipt.Stage != "" && (!strings.HasPrefix(receipt.Stage, ".portscoop-main-") || filepath.Base(receipt.Stage) != receipt.Stage)) {
			return fmt.Errorf("invalid owned bootstrap receipt")
		}
		if receipt.Stage != "" {
			stage, err := runtimeinstall.CheckPath(paths.Root, "scoop/buckets/"+receipt.Stage)
			if err != nil {
				return err
			}
			// Only the generated path named by this receipt is retry-owned.
			if err = os.RemoveAll(stage); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if info, err := os.Lstat(main); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("Main conflicts with an existing file")
		}
		// Require a real local Git directory, not a worktree file referring out.
		dotgit, err := runtimeinstall.CheckPath(paths.Root, "scoop/buckets/main/.git")
		if err != nil {
			return err
		}
		if info, err := os.Stat(dotgit); err != nil || !info.IsDir() {
			return fmt.Errorf("Main is not an owned Git bucket; existing files preserved")
		}
		url, err := b.git(ctx, paths, git, main, "config", "--local", "--get", "remote.origin.url")
		if err != nil || !officialMain(url) {
			return fmt.Errorf("Main has a different source; existing bucket preserved")
		}
		if receipt.Complete && receipt.Stage == "" {
			return nil
		}
		return runtimeinstall.Write(paths.Root, bootstrapReceiptPath, bootstrapReceipt{Schema: 1, Repository: MainRepository, Revision: MainRevision, Complete: true})
	} else if !os.IsNotExist(err) {
		return err
	}
	stage, err := os.MkdirTemp(paths.Buckets, ".portscoop-main-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	receipt = bootstrapReceipt{Schema: 1, Repository: MainRepository, Revision: MainRevision, Stage: filepath.Base(stage)}
	if err = runtimeinstall.Write(paths.Root, bootstrapReceiptPath, receipt); err != nil {
		return err
	}
	commands := [][]string{{"init", stage}, {"remote", "add", "origin", MainRepository}, {"fetch", "--depth=1", "origin", MainRevision}, {"checkout", "-B", "master", "FETCH_HEAD"}, {"config", "branch.master.remote", "origin"}, {"config", "branch.master.merge", "refs/heads/master"}}
	for _, args := range commands {
		if _, err = b.git(ctx, paths, git, stage, args...); err != nil {
			return fmt.Errorf("Main bootstrap %s: %w", args[0], err)
		}
	}
	head, err := b.git(ctx, paths, git, stage, "rev-parse", "HEAD")
	if err != nil || head != MainRevision {
		return fmt.Errorf("Main checkout does not match pinned revision")
	}
	if _, err = runtimeinstall.CheckPath(paths.Root, "scoop/buckets/main"); err != nil {
		return err
	}
	if err = os.Rename(stage, main); err != nil {
		return err
	}
	receipt.Stage = ""
	receipt.Complete = true
	return runtimeinstall.Write(paths.Root, bootstrapReceiptPath, receipt)
}
