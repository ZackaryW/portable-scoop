package compatibility

import (
	"encoding/json"
	"fmt"
)

type Download struct {
	URL        string `json:"url"`
	Name       string `json:"name"`
	Hash       string `json:"hash"`
	Format     string `json:"format"`
	ExtractDir string `json:"extract_dir,omitempty"`
	ExtractTo  string `json:"extract_to,omitempty"`
}

func extractionValues(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if s, err := stringValue(raw); err == nil {
		return []string{s}, nil
	}
	var values []*string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	out := make([]string, len(values))
	for i, v := range values {
		if v != nil {
			out[i] = *v
		}
	}
	return out, nil
}
func extractionField(raw json.RawMessage) (result, error) {
	vs, err := extractionValues(raw)
	if err != nil {
		return unsupported("invalid extraction path list")
	}
	for _, v := range vs {
		if v != "" && v != "." && !relative(v) {
			return unsupported("extraction path must stay inside package")
		}
	}
	return keep(raw)
}
func compileDownloads(m Manifest) ([]Download, error) {
	urls, err := stringsValue(m["url"])
	if err != nil {
		return nil, err
	}
	hashes, err := stringsValue(m["hash"])
	if err != nil || len(hashes) != len(urls) {
		return nil, fmt.Errorf("download digest pairing mismatch")
	}
	dirs, err := extractionValues(m["extract_dir"])
	if err != nil {
		return nil, err
	}
	tos, err := extractionValues(m["extract_to"])
	if err != nil {
		return nil, err
	}
	if len(dirs) > len(urls) || len(tos) > len(urls) {
		return nil, fmt.Errorf("more extraction declarations than downloads")
	}
	out := make([]Download, len(urls))
	for i, value := range urls {
		u, name, format, err := downloadName(value)
		if err != nil {
			return nil, err
		}
		d := Download{URL: u, Name: name, Hash: hashes[i], Format: format}
		if i < len(dirs) {
			d.ExtractDir = dirs[i]
		}
		if i < len(tos) {
			d.ExtractTo = tos[i]
		}
		if format == "file" && (d.ExtractDir != "" || d.ExtractTo != "") {
			return nil, fmt.Errorf("inert EXE payload cannot have extraction directives")
		}
		out[i] = d
	}
	return out, nil
}
