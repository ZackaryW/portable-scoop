package registry

import (
	"context"
	"fmt"
	"sort"

	"portscoop/internal/environment"
)

type Listing struct {
	Alias       string
	Entry       Entry
	Default     bool
	Status      string
	DisplayName string
}

// List bounds time spent probing unavailable media and keeps at most four
// filesystem probes in flight. No metadata is removed or written by a listing.
func (m Manager) List(ctx context.Context) ([]Listing, error) {
	d, err := m.Store.Read()
	if err != nil {
		return nil, err
	}
	aliases := make([]string, 0, len(d.Entries))
	for alias := range d.Entries {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	rows := make([]Listing, len(aliases))
	entries := make([]Entry, len(aliases))
	for i, alias := range aliases {
		entry := d.Entries[alias]
		entries[i] = entry
		rows[i] = Listing{Alias: alias, Entry: entry, Default: entry.ID == d.DefaultID, Status: "unavailable (inspection deadline exceeded)"}
	}
	type result struct {
		index        int
		status, name string
	}
	results := make(chan result, len(rows))
	jobs := make(chan int, len(rows))
	for i := range rows {
		jobs <- i
	}
	close(jobs)
	for range min(4, len(rows)) {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case i, ok := <-jobs:
					if !ok {
						return
					}
					id, _, err := environment.Load(m.filesystem(), entries[i].Path)
					item := result{index: i, status: "available", name: id.Name}
					if err != nil {
						item.status = fmt.Sprintf("unavailable (%v)", err)
					} else if id.ID != entries[i].ID {
						item.status = "identity-mismatch"
					}
					results <- item
				}
			}
		}()
	}
	for range rows {
		select {
		case <-ctx.Done():
			return rows, nil
		case item := <-results:
			rows[item.index].Status = item.status
			rows[item.index].DisplayName = item.name
		}
	}
	return rows, nil
}
