//go:build windows

// Package localvol describes supported local volume locations without scanning
// directory trees or connecting to network drives.
package localvol

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

type Volume struct{ Root, ID string }
type Source interface {
	Volumes(context.Context) ([]Volume, error)
}
type System struct{}

func Identity(path string) (string, error) {
	root := filepath.VolumeName(path) + string(filepath.Separator)
	if len(filepath.VolumeName(path)) != 2 {
		return "", fmt.Errorf("only local drive volumes are supported")
	}
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "", err
	}
	var name [128]uint16
	if err := windows.GetVolumeNameForVolumeMountPoint(ptr, &name[0], uint32(len(name))); err != nil {
		return "", err
	}
	return windows.UTF16ToString(name[:]), nil
}

func (System) Volumes(ctx context.Context) ([]Volume, error) {
	var buffer [256]uint16
	n, err := windows.GetLogicalDriveStrings(uint32(len(buffer)), &buffer[0])
	if err != nil {
		return nil, err
	}
	if n >= uint32(len(buffer)) {
		return nil, fmt.Errorf("unexpected logical drive list length")
	}
	var roots []string
	for _, root := range strings.Split(string(utf16.Decode(buffer[:n])), "\x00") {
		if root != "" {
			roots = append(roots, root)
		}
	}
	budget, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	jobs := make(chan string, len(roots))
	for _, root := range roots {
		jobs <- root
	}
	close(jobs)
	results := make(chan *Volume, len(roots))
	for range min(4, len(roots)) {
		go func() {
			for {
				select {
				case <-budget.Done():
					return
				case root, ok := <-jobs:
					if !ok {
						return
					}
					ptr, err := windows.UTF16PtrFromString(root)
					if err != nil {
						results <- nil
						continue
					}
					kind := windows.GetDriveType(ptr)
					if kind != windows.DRIVE_FIXED && kind != windows.DRIVE_REMOVABLE {
						results <- nil
						continue
					}
					id, err := Identity(root)
					if err != nil {
						results <- nil
						continue
					}
					results <- &Volume{Root: root, ID: id}
				}
			}
		}()
	}
	var found []Volume
	for range roots {
		select {
		case <-budget.Done():
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return found, nil
		case volume := <-results:
			if volume != nil {
				found = append(found, *volume)
			}
		}
	}
	return found, nil
}
