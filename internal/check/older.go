package check

import (
	"path"
	"slices"

	"github.com/ZachCurry13/isoshelf/internal/version"
)

// markOlderCopies notes files that have a newer copy of the same entry on
// the target. Files without a version in their name are left alone, since
// their age can't be told.
func markOlderCopies(items []Item) {
	byEntry := map[string][]*Item{}
	for i := range items {
		if it := &items[i]; it.Entry != nil && it.Path != "" && it.Version != "" {
			byEntry[it.Entry.ID] = append(byEntry[it.Entry.ID], it)
		}
	}
	for _, group := range byEntry {
		if len(group) < 2 {
			continue
		}
		newest := slices.MaxFunc(group, func(a, b *Item) int { return version.Compare(a.Version, b.Version) })
		for _, it := range group {
			if it == newest || version.Compare(it.Version, newest.Version) >= 0 {
				continue
			}
			it.Older = true
			if it.Note == "" {
				it.Note = "older copy; the newest here is " + path.Base(newest.Path)
			}
		}
	}
}
