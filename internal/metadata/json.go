// Package metadata provides strict decoding for versioned metadata documents.
package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Decode rejects unknown fields and trailing documents instead of accepting a
// partially understood configuration.
func Decode(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := d.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("metadata must contain exactly one JSON document")
	}
	return nil
}

// RelativePath accepts portable root-relative paths on every build platform.
func RelativePath(value string) bool {
	if value == "" || strings.ContainsAny(value, ":\x00") {
		return false
	}
	value = strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(value, "/") {
		return false
	}
	for part := range strings.SplitSeq(value, "/") {
		if part == ".." || part == "." || part == "" || strings.TrimRight(part, " .") != part {
			return false
		}
	}
	return filepath.IsLocal(filepath.FromSlash(value))
}
