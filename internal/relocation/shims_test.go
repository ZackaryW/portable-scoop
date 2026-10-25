package relocation

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/testutil"
)

func nativeShimFixture(t *testing.T) environment.Paths {
	t.Helper()
	paths, _ := environment.PathsAt(testutil.TempDir(t))
	stub := make([]byte, 256)
	copy(stub, "MZ")
	binary.LittleEndian.PutUint32(stub[60:64], 64)
	copy(stub[64:], "PE\x00\x00")
	writeFixture(t, filepath.Join(filepath.Dir(filepath.Dir(paths.Engine)), "supporting", "shims", "kiennq", "shim.exe"), stub)
	writeFixture(t, filepath.Join(paths.Shims, "native.exe"), stub)
	target := filepath.Join(paths.Apps, "fixture", "1", "app.exe")
	writeFixture(t, target, []byte("owned target"))
	writeFixture(t, filepath.Join(paths.Shims, "native.shim"), []byte("path = \""+target+"\"\n"))
	if err := os.MkdirAll(paths.State, 0700); err != nil {
		t.Fatal(err)
	}
	if err := (Manager{}).Capture(paths); err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestOwnedNativeDescriptorAndCase(t *testing.T) {
	paths := nativeShimFixture(t)
	got, err := (Manager{}).ResolveOwnedShim(paths, "NaTiVe")
	if err != nil || got.Handler != "native" || got.Target != filepath.Join(paths.Apps, "fixture", "1", "app.exe") {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestOwnedEquivalentWrapperFamily(t *testing.T) {
	paths := relocationFixture(t)
	got, err := (Manager{}).ResolveOwnedShim(paths, "tool")
	if err != nil || got.Handler != "batch" || !strings.Contains(got.Target, `\1.0\`) {
		t.Fatalf("%+v %v", got, err)
	}
	if err := os.Remove(filepath.Join(paths.Apps, "tool", "1.0", "tools", "tool.cmd")); err != nil {
		t.Fatal(err)
	}
	if _, err := (Manager{}).ResolveOwnedShim(paths, "tool"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing target: %v", err)
	}
}

func TestOwnedShimRejectsConflictsAndFixedArgs(t *testing.T) {
	for _, kind := range []string{"conflict", "fixed native", "fixed batch", "unrecorded", "edited", "missing native metadata"} {
		t.Run(kind, func(t *testing.T) {
			paths := nativeShimFixture(t)
			name, want := "native", ""
			switch kind {
			case "conflict":
				other := filepath.Join(paths.Apps, "fixture", "1", "other.cmd")
				writeFixture(t, other, []byte("@echo other"))
				writeFixture(t, filepath.Join(paths.Shims, "native.cmd"), []byte("@rem "+other+"\n@\""+other+"\"  %*"))
				want = "conflicting"
			case "fixed native":
				path := filepath.Join(paths.Shims, "native.shim")
				data, _ := os.ReadFile(path)
				writeFixture(t, path, append(data, []byte("args = --fixed\n")...))
				want = "fixed"
			case "fixed batch":
				name = "batch"
				target := filepath.Join(paths.Apps, "fixture", "1", "other.cmd")
				writeFixture(t, target, []byte("@echo other"))
				writeFixture(t, filepath.Join(paths.Shims, "batch.cmd"), []byte("@rem "+target+"\n@\""+target+"\" --fixed %*"))
				want = "fixed"
			case "unrecorded":
				writeFixture(t, filepath.Join(paths.Shims, "new.shim"), []byte("path = \""+filepath.Join(paths.Apps, "fixture", "1", "app.exe")+"\""))
				want = "inventory changed"
			case "edited":
				path := filepath.Join(paths.Shims, "native.shim")
				data, _ := os.ReadFile(path)
				writeFixture(t, path, append(data, '\n'))
				want = "changed"
			case "missing native metadata":
				if err := os.Remove(filepath.Join(paths.Shims, "native.shim")); err != nil {
					t.Fatal(err)
				}
				want = "incomplete"
			}
			if kind != "unrecorded" && kind != "edited" {
				if err := (Manager{}).Capture(paths); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := (Manager{}).ResolveOwnedShim(paths, name); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("wanted %s, got %v", want, err)
			}
		})
	}
}

func TestLogicalNamesAndNoFallback(t *testing.T) {
	paths := nativeShimFixture(t)
	for _, name := range []string{"../native", `C:\native.exe`, `native\x`, "native/../x", "NUL", "CON.txt", "COM1", "Lpt9.log", "native.", "", "foo:bar", " x", "a\x00b"} {
		if _, err := (Manager{}).ResolveOwnedShim(paths, name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	outside := testutil.TempDir(t)
	writeFixture(t, filepath.Join(outside, "outside.exe"), []byte("not owned"))
	t.Setenv("PATH", outside)
	t.Setenv("SCOOP", outside)
	if _, err := (Manager{}).ResolveOwnedShim(paths, "outside"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("outside fallback: %v", err)
	}
}

func TestRuntimeFormatsRemainExplicit(t *testing.T) {
	for _, ext := range []string{".py", ".jar", ".sh", ".ps1"} {
		t.Run(ext, func(t *testing.T) {
			paths, _ := environment.PathsAt(testutil.TempDir(t))
			target := filepath.Join(paths.Apps, "fixture", "1", "app"+ext)
			writeFixture(t, target, []byte("fixture"))
			if err := os.MkdirAll(paths.State, 0700); err != nil {
				t.Fatal(err)
			}
			var content string
			switch ext {
			case ".py":
				content = "@rem " + target + "\n@python \"" + target + "\"  %*"
			case ".jar":
				content = "@rem " + target + "\n@pushd fixture\n@java -jar fixture\n@popd"
			case ".sh":
				content = "#!/bin/sh\n# " + target + "\nif [ $WSL_INTEROP ]\nthen\nfixture\nelse\nfixture\nfi"
			case ".ps1":
				content = "# " + target + "\n$path = \"" + target + "\"\nif ($MyInvocation.ExpectingInput) { $input | & $path  @args } else { & $path  @args }\nexit $LASTEXITCODE"
			}
			name := "app.cmd"
			if ext == ".sh" {
				name = "app"
			}
			if ext == ".ps1" {
				name = "app.ps1"
			}
			writeFixture(t, filepath.Join(paths.Shims, name), []byte(content))
			if err := (Manager{}).Capture(paths); err != nil {
				t.Fatal(err)
			}
			got, err := (Manager{}).ResolveOwnedShim(paths, "app")
			if ext == ".ps1" {
				if err != nil || got.Handler != "powershell" {
					t.Fatalf("%+v %v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "runtime handler") {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
}

func TestEscapedTargetRejected(t *testing.T) {
	paths := relocationFixture(t)
	link := filepath.Join(paths.Apps, "tool", "current")
	if err := removeLink(link); err != nil {
		t.Fatal(err)
	}
	if err := createJunction(link, testutil.TempDir(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := (Manager{}).ResolveOwnedShim(paths, "tool"); err == nil {
		t.Fatal("escaped link accepted")
	}
}
