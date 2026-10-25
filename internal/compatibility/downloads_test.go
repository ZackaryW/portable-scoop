package compatibility

import (
	"reflect"
	"strings"
	"testing"
)

func TestFrozenDownloads(t *testing.T) {
	m := fixture(t, map[string]any{"url": []string{"https://example.test/download?name=wrong.exe#/Tool.zip", "https://example.test/binary.exe#/run.exe"}, "hash": []string{strings.Repeat("a", 64), "sha1:" + strings.Repeat("b", 40)}, "extract_dir": []any{"content", nil}, "extract_to": []string{"tools", ""}})
	p, err := Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	want := []Download{{URL: "https://example.test/download?name=wrong.exe", Name: "Tool.zip", Hash: strings.Repeat("a", 64), Format: "zip", ExtractDir: "content", ExtractTo: "tools"}, {URL: "https://example.test/binary.exe", Name: "run.exe", Hash: "sha1:" + strings.Repeat("b", 40), Format: "file"}}
	if !reflect.DeepEqual(p.Downloads, want) {
		t.Fatalf("%+v", p.Downloads)
	}
	again, err := Compile(p.Manifest)
	if err != nil || !reflect.DeepEqual(p.Downloads, again.Downloads) || !reflect.DeepEqual(p.Manifest, again.Manifest) {
		t.Fatal("download plan not idempotent", err)
	}
}
func TestDownloadNameBoundaries(t *testing.T) {
	for _, url := range []string{"https://example.test/app.exe", "https://example.test/file.zip?name=evil.exe", "https://example.test/a%20b.zip", "https://example.test/get?download=1#/tool.exe"} {
		if _, err := Compile(fixture(t, map[string]any{"url": url})); err != nil {
			t.Fatal(url, err)
		}
	}
	for _, url := range []string{"https://example.test/a.zip#/../evil.zip", "https://example.test/a.zip#//host/evil.zip", "https://example.test/a.zip#/%2fescape.zip", "https://example.test/a.zip#/NUL.exe", "https://example.test/a.zip#/a:stream.exe", "https://example.test/a.zip#wrong.zip", "https://example.test/a%2fb.zip", "file:///a.zip", "https://user:pass@example.test/a.zip", "https://example.test/a.msi"} {
		p, err := Compile(fixture(t, map[string]any{"url": url}))
		if err == nil || p.Downloads != nil {
			t.Fatal("accepted", url)
		}
	}
	for _, fields := range []map[string]any{{"url": []string{"https://one.test/a.zip", "https://two.test/A.zip"}, "hash": []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}}, {"extract_to": []string{"one", "two"}}, {"url": "https://example.test/app.exe", "extract_dir": "."}} {
		if _, err := Compile(fixture(t, fields)); err == nil {
			t.Fatal("accepted invalid pairing", fields)
		}
	}
}
