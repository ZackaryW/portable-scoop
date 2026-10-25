package selfupdate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// One invocation reuses identical pinned bytes; each target still independently
// verifies its checksum and extracted inventory in its own private staging area.
type cachedDownloads struct {
	source    Downloader
	directory string
	files     map[string]string
}

func (c *cachedDownloads) close() {
	if c.directory != "" {
		_ = os.RemoveAll(c.directory)
	}
}
func (c *cachedDownloads) Download(ctx context.Context, url string, out io.Writer, limit int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.directory == "" {
		dir, err := os.MkdirTemp("", "portscoop-update-downloads-")
		if err != nil {
			return err
		}
		c.directory = dir
		c.files = map[string]string{}
	}
	p, ok := c.files[url]
	if !ok {
		p = filepath.Join(c.directory, fmt.Sprintf("%d", len(c.files)))
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		err = c.source.Download(ctx, url, f, limit)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(p)
			return err
		}
		c.files[url] = p
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > limit {
		return fmt.Errorf("cached download exceeds bound")
	}
	_, err = io.Copy(out, f)
	return err
}
