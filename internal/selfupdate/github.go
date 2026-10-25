package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const repository = "ZackaryW/portable-scoop"
const asset = "portscoop-windows-amd64.zip"
const ArchiveLimit int64 = 512 * 1024 * 1024

type GitHub struct{ client *http.Client }

func NewGitHub() *GitHub {
	return &GitHub{&http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || req.URL.Scheme != "https" || req.URL.User != nil || req.URL.Port() != "" {
			return fmt.Errorf("unsupported release redirect")
		}
		h := req.URL.Hostname()
		if h != "github.com" && h != "api.github.com" && h != "release-assets.githubusercontent.com" && h != "objects.githubusercontent.com" {
			return fmt.Errorf("release redirect outside GitHub assets")
		}
		return nil
	}}}
}
func (g *GitHub) Download(ctx context.Context, location string, w io.Writer, limit int64) error {
	u, err := url.Parse(location)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Host != "api.github.com" && u.Host != "github.com") {
		return fmt.Errorf("invalid official release URL")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", location, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Portscoop-self-update/1")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("release request failed: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return fmt.Errorf("release response exceeds limit")
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("release response exceeds limit")
	}
	return nil
}
func (g *GitHub) Resolve(ctx context.Context, version string) (Release, error) {
	endpoint := "https://api.github.com/repos/" + repository + "/releases/latest"
	if version != "" {
		version = normalize(version)
		if !semver.IsValid(version) || semver.Canonical(version) != version {
			return Release{}, fmt.Errorf("version must be a complete semantic release tag")
		}
		endpoint = "https://api.github.com/repos/" + repository + "/releases/tags/" + url.PathEscape(version)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var b strings.Builder
	if err := g.Download(ctx, endpoint, &b, 2*1024*1024); err != nil {
		return Release{}, err
	}
	var response struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal([]byte(b.String()), &response); err != nil {
		return Release{}, err
	}
	if !semver.IsValid(response.Tag) || semver.Canonical(response.Tag) != response.Tag || response.Draft || (version == "" && (response.Prerelease || semver.Prerelease(response.Tag) != "")) || (version != "" && response.Tag != version) {
		return Release{}, fmt.Errorf("unsupported published release")
	}
	base := "https://github.com/" + repository + "/releases/download/" + response.Tag + "/"
	want := map[string]int{asset: 0, asset + ".sha256": 0}
	for _, a := range response.Assets {
		if _, ok := want[a.Name]; ok {
			want[a.Name]++
			if a.URL != base+a.Name || a.Size <= 0 || a.Size > ArchiveLimit {
				return Release{}, fmt.Errorf("invalid release asset")
			}
		}
	}
	for _, n := range want {
		if n != 1 {
			return Release{}, fmt.Errorf("release requires one ZIP and checksum asset")
		}
	}
	return Release{response.Tag, base + asset, base + asset + ".sha256"}, nil
}
