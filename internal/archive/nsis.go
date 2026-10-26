package archive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Detection is a bounded content gate, not validation. The forced NSIS parser
// must still accept the complete archive and every stream must finish cleanly.
func isNSIS(r io.ReaderAt) bool {
	b := make([]byte, 1<<20)
	n, _ := r.ReadAt(b, 0)
	b = b[:n]
	return len(b) >= 2 && string(b[:2]) == "MZ" && bytes.Contains(b, []byte("\xef\xbe\xad\xdeNullsoftInst"))
}

type nsisGroup struct {
	name  string // exact native selection, before separator normalization
	dir   bool
	sizes []int64 // -1 only for a singleton with unknown size
}

func parseNSIS(data []byte, limits Limits) ([]nsisGroup, error) {
	if len(data) > nsisInventoryLimit || !utf8.Valid(data) || bytes.ContainsRune(data, '\x00') {
		return nil, fmt.Errorf("invalid NSIS inventory encoding or length")
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	var groups []nsisGroup
	indices := map[string]int{}
	paths := map[string]entry{}
	count, total := 0, int64(0)
	for _, block := range strings.Split(strings.Trim(text, "\n"), "\n\n") {
		fields := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, " = ")
			if !ok {
				return nil, fmt.Errorf("ambiguous NSIS inventory line")
			}
			switch key {
			case "Path", "Size", "Packed Size", "Modified", "Attributes", "Method", "Solid", "Offset", "Folder":
			default:
				return nil, fmt.Errorf("unsupported NSIS metadata %q", key)
			}
			if _, exists := fields[key]; exists {
				return nil, fmt.Errorf("duplicate NSIS metadata %q", key)
			}
			fields[key] = value
		}
		name := fields["Path"]
		normal := strings.ReplaceAll(name, `\`, "/")
		// 7-Zip reserves a leading @ for response-file selectors even where
		// option and wildcard parsing are disabled; never pass one as a name.
		if !SafePath(normal) || strings.HasPrefix(name, "@") {
			return nil, fmt.Errorf("unsafe NSIS member %q", name)
		}
		attributes, ok := fields["Attributes"]
		if !ok || strings.Trim(attributes, "RAHSD ") != "" {
			return nil, fmt.Errorf("unsupported NSIS attributes")
		}
		dir := strings.Contains(attributes, "D")
		if folder, ok := fields["Folder"]; ok {
			if folder != "+" && folder != "-" {
				return nil, fmt.Errorf("invalid NSIS folder flag")
			}
			if strings.Contains(attributes, "D") && folder != "+" {
				return nil, fmt.Errorf("conflicting NSIS member type")
			}
			dir = folder == "+"
		}
		value, ok := fields["Size"]
		if !ok {
			return nil, fmt.Errorf("missing NSIS size field")
		}
		size := int64(-1)
		if value != "" {
			var err error
			size, err = strconv.ParseInt(value, 10, 64)
			if err != nil || size < 0 || strconv.FormatInt(size, 10) != value {
				return nil, fmt.Errorf("invalid NSIS size")
			}
		}
		count++
		if count > limits.Files || size > limits.MemberBytes || size > limits.TotalBytes-total {
			return nil, fmt.Errorf("NSIS inventory resource limit exceeded")
		}
		if size >= 0 {
			total += size
		}
		if dir && size != 0 {
			return nil, fmt.Errorf("invalid NSIS directory size")
		}
		// Validate implicit parents before any native member is requested.
		parts := strings.Split(normal, "/")
		for i := range parts {
			p := strings.Join(parts[:i+1], "/")
			key := strings.ToLower(p)
			isDir := i < len(parts)-1 || dir
			if old, exists := paths[key]; exists {
				if old.name != p || old.dir != isDir {
					return nil, fmt.Errorf("NSIS path collision %q", p)
				}
			} else {
				paths[key] = entry{p, isDir}
			}
		}
		if index, ok := indices[strings.ToLower(normal)]; ok {
			g := &groups[index]
			if g.name != name || g.dir != dir || size < 0 || g.sizes[0] < 0 {
				return nil, fmt.Errorf("ambiguous NSIS duplicate %q", name)
			}
			g.sizes = append(g.sizes, size)
		} else {
			indices[strings.ToLower(normal)] = len(groups)
			groups = append(groups, nsisGroup{name: name, dir: dir, sizes: []int64{size}})
		}
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("empty NSIS inventory")
	}
	return groups, nil
}

// segmentWriter discards earlier exact-name entries, retaining the last only.
// All bytes, including discarded segments, are checked against the inventory.
type segmentWriter struct {
	w                 io.Writer
	skip, keep, count int64
	unknown           bool
}

func (w *segmentWriter) Write(p []byte) (int, error) {
	original := len(p)
	if !w.unknown && int64(len(p)) > w.skip+w.keep-w.count {
		return 0, fmt.Errorf("NSIS member exceeds listed sizes")
	}
	if w.count < w.skip {
		discard := min(int64(len(p)), w.skip-w.count)
		w.count += discard
		p = p[discard:]
	}
	if len(p) != 0 {
		n, err := w.w.Write(p)
		w.count += int64(n)
		if err != nil {
			return original - len(p) + n, err
		}
		if n != len(p) {
			return original - len(p) + n, io.ErrShortWrite
		}
	}
	return original, nil
}

func (s *staging) extractNSIS(source string, reader NSISReader) error {
	if reader == nil {
		return fmt.Errorf("missing NSIS reader")
	}
	ctx, cancel := context.WithTimeout(s.ctx, nsisExtractionTimeout)
	defer cancel()
	data, err := reader.List(ctx, source)
	if err != nil {
		return err
	}
	groups, err := parseNSIS(data, s.limits)
	if err != nil {
		return err
	}
	consumed := int64(0)
	for _, g := range groups {
		if err := ctx.Err(); err != nil {
			return err
		}
		if g.dir {
			if err := s.add(g.name, os.ModeDir|0700, 0, nil); err != nil {
				return err
			}
			continue
		}
		if err := s.extractNSISGroup(ctx, source, reader, g, &consumed); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (s *staging) extractNSISGroup(ctx context.Context, source string, reader NSISReader, g nsisGroup, consumed *int64) error {
	f, err := os.CreateTemp(s.root, ".nsis-member-")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	w := &segmentWriter{w: f, keep: g.sizes[len(g.sizes)-1], unknown: g.sizes[0] < 0}
	for _, size := range g.sizes[:len(g.sizes)-1] {
		w.skip += size
	}
	limit := w.skip + w.keep
	if w.unknown {
		limit = min(s.limits.MemberBytes, s.limits.TotalBytes-*consumed)
	}
	if limit < 0 || limit > s.limits.TotalBytes-*consumed {
		return fmt.Errorf("NSIS aggregate byte limit exceeded")
	}
	// Keep a second cap here so alternative readers cannot bypass accounting.
	out := &cappedOutput{w: w, left: limit, cancel: func() {}}
	if err = reader.Stream(ctx, source, g.name, out, limit); err != nil {
		return err
	}
	if out.err != nil {
		return out.err
	}
	if !w.unknown && w.count != limit {
		return fmt.Errorf("NSIS member size differs: %s", g.name)
	}
	*consumed += w.count
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	return s.add(g.name, 0600, w.count-w.skip, f)
}
