// Package toolbundle prepares pinned build-time tool distributions without
// executing their contents. Runtime installation owns the resulting inventory.
package toolbundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portscoop/internal/archive"
)

type Tool struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	URL            string   `json:"url"`
	SHA256         string   `json:"sha256"`
	Format         string   `json:"format"`
	Required       []string `json:"required"`
	Retain         []string `json:"retain,omitempty"`
	Source         string   `json:"source"`
	SourcePackages string   `json:"source_packages"`
}
type Pins struct {
	SchemaVersion int    `json:"schema_version"`
	Tools         []Tool `json:"tools"`
}
type Inventory struct {
	Pins
	Artifacts map[string]string `json:"artifacts"`
}

const maxArtifactBytes = 256 << 20

func ReadPins(path string) (Pins, error) {
	var p Pins
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if err = d.Decode(&p); err != nil {
		return p, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return p, fmt.Errorf("trailing pin content")
	}
	return p, p.Validate()
}
func (p Pins) Validate() error {
	if p.SchemaVersion != 1 || len(p.Tools) == 0 {
		return fmt.Errorf("invalid tool pins")
	}
	seen := map[string]bool{}
	for _, t := range p.Tools {
		if !archive.SafePath(t.Name) || strings.ContainsAny(t.Name, "/\\") || seen[strings.ToLower(t.Name)] || t.Version == "" {
			return fmt.Errorf("invalid tool identity")
		}
		seen[strings.ToLower(t.Name)] = true
		h, err := hex.DecodeString(t.SHA256)
		if err != nil || len(h) != 32 {
			return fmt.Errorf("invalid tool SHA-256")
		}
		for _, raw := range []string{t.URL, t.Source, t.SourcePackages} {
			u, e := url.Parse(raw)
			if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
				return fmt.Errorf("invalid tool provenance URL")
			}
		}
		if t.Format != "zip" && t.Format != "7z" {
			return fmt.Errorf("unsupported seed archive")
		}
		if len(t.Required) == 0 {
			return fmt.Errorf("missing required tool files")
		}
		for _, name := range append(append([]string{}, t.Required...), t.Retain...) {
			if !archive.SafePath(name) {
				return fmt.Errorf("unsafe tool path")
			}
		}
		if len(t.Retain) > 0 {
			retained := map[string]bool{}
			for _, name := range t.Retain {
				if retained[strings.ToLower(name)] {
					return fmt.Errorf("duplicate retained path")
				}
				retained[strings.ToLower(name)] = true
			}
			for _, name := range t.Required {
				if !retained[strings.ToLower(name)] {
					return fmt.Errorf("required file omitted from bundle")
				}
			}
		}
	}
	return nil
}
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	i, err := f.Stat()
	if err != nil || !i.Mode().IsRegular() {
		return "", fmt.Errorf("tool artifact must be regular")
	}
	if i.Size() > maxArtifactBytes {
		return "", fmt.Errorf("tool artifact exceeds size limit")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// Prepare rejects existing destinations and publishes only a complete bundle.
// Cached bytes are rehashed on every use; a bad cache entry is never executed,
// adopted or silently overwritten. Neither an installer nor host tool is run.
func Prepare(ctx context.Context, p Pins, cache, destination string) (Inventory, error) {
	result := Inventory{Pins: p, Artifacts: map[string]string{}}
	if err := p.Validate(); err != nil {
		return result, err
	}
	cache, err := filepath.Abs(cache)
	if err != nil {
		return result, err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return result, err
	}
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		return result, fmt.Errorf("tool destination must not exist")
	}
	if err = os.MkdirAll(cache, 0700); err != nil {
		return result, err
	}
	if err = archive.Confined(cache, cache); err != nil {
		return result, err
	}
	parent := filepath.Dir(destination)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return result, err
	}
	if err = archive.Confined(parent, parent); err != nil {
		return result, err
	}
	stage, err := os.MkdirTemp(parent, ".portscoop-tools-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(stage)
	bundle := filepath.Join(stage, "bundle")
	if err = os.Mkdir(bundle, 0700); err != nil {
		return result, err
	}
	client := &http.Client{Timeout: 3 * time.Minute}
	for _, t := range p.Tools {
		cached := filepath.Join(cache, t.SHA256)
		if err = archive.Confined(cache, cached); err != nil {
			return result, err
		}
		if _, err = os.Lstat(cached); os.IsNotExist(err) {
			req, e := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
			if e != nil {
				return result, e
			}
			response, e := client.Do(req)
			if e != nil {
				return result, e
			}
			if response.StatusCode != http.StatusOK {
				response.Body.Close()
				return result, fmt.Errorf("tool download HTTP %d", response.StatusCode)
			}
			tmp, e := os.CreateTemp(cache, ".download-")
			if e != nil {
				response.Body.Close()
				return result, e
			}
			n, copyErr := io.Copy(tmp, io.LimitReader(response.Body, maxArtifactBytes+1))
			response.Body.Close()
			closeErr := tmp.Close()
			if copyErr != nil || closeErr != nil || n > maxArtifactBytes {
				os.Remove(tmp.Name())
				return result, fmt.Errorf("tool download failed or exceeded limit")
			}
			h, e := hashFile(tmp.Name())
			if e != nil || h != t.SHA256 {
				os.Remove(tmp.Name())
				return result, fmt.Errorf("tool download SHA-256 mismatch")
			}
			if e = os.Rename(tmp.Name(), cached); e != nil {
				os.Remove(tmp.Name())
				return result, e
			}
		} else if err != nil {
			return result, err
		}
		h, e := hashFile(cached)
		if e != nil || h != t.SHA256 {
			return result, fmt.Errorf("cached tool SHA-256 mismatch: %s", t.Name)
		}
		source := filepath.Join(stage, t.Name+".archive")
		if err = copyFile(cached, source); err != nil {
			return result, err
		}
		extracted := filepath.Join(stage, t.Name+"-extracted")
		if err = os.Mkdir(extracted, 0700); err != nil {
			return result, err
		}
		if err = archive.Extract(ctx, stage, archive.Request{Source: source, Destination: extracted, Format: t.Format}); err != nil {
			return result, err
		}
		for _, name := range t.Required {
			if _, err = hashFile(filepath.Join(extracted, filepath.FromSlash(name))); err != nil {
				return result, fmt.Errorf("required %s/%s: %w", t.Name, name, err)
			}
		}
		output := filepath.Join(bundle, t.Name)
		if len(t.Retain) == 0 {
			if err = os.Rename(extracted, output); err != nil {
				return result, err
			}
		} else {
			for _, name := range t.Retain {
				if err = copyFile(filepath.Join(extracted, filepath.FromSlash(name)), filepath.Join(output, filepath.FromSlash(name))); err != nil {
					return result, err
				}
			}
		}
	}
	if err = filepath.WalkDir(bundle, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if e = archive.Confined(bundle, path); e != nil {
			return e
		}
		h, e := hashFile(path)
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(bundle, path)
		result.Artifacts[filepath.ToSlash(rel)] = h
		return nil
	}); err != nil {
		return result, err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, err
	}
	if err = os.WriteFile(filepath.Join(bundle, "tools.json"), append(data, '\n'), 0600); err != nil {
		return result, err
	}
	if err = os.Rename(bundle, destination); err != nil {
		return result, err
	}
	return result, nil
}
