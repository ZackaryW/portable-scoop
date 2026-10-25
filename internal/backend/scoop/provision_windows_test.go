package scoop

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestVerifyReleaseThroughWindowsShortPath(t *testing.T) {
	release := releaseFixture(t)
	input, err := windows.UTF16PtrFromString(release)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	n, err := windows.GetShortPathName(input, &buffer[0], uint32(len(buffer)))
	if err != nil || n == 0 || n >= uint32(len(buffer)) {
		t.Fatalf("short release path: %d %v", n, err)
	}
	alias := windows.UTF16ToString(buffer[:n])
	if strings.EqualFold(alias, release) {
		t.Skip("temporary volume does not expose an 8.3 alias")
	}
	if _, err := VerifyRelease(alias); err != nil {
		t.Fatalf("valid release rejected through Windows alias %s: %v", alias, err)
	}
}
