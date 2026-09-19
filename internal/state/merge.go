package state

import (
	"maps"
	"slices"
)

// Several things can change a folder's state while isoshelf runs: a download
// that takes an hour, and the user removing or identifying files meanwhile.
// Each works on its own copy. Instead of saving that copy over the file (and
// losing whatever the others did), a writer keeps the copy it started from,
// and Merge carries only its own changes over to the state as it is on disk
// now.

// Clone returns a copy of s that can be changed without touching s.
func (s *State) Clone() *State {
	c := *s
	c.Files = maps.Clone(s.Files)
	c.Tracks = maps.Clone(s.Tracks)
	c.History = slices.Clone(s.History)
	c.Past = slices.Clone(s.Past)
	return &c
}

// Merge applies the changes made between base and changed to onto: file
// records and track settings added, changed or removed; notes about images
// that left the folder, added or dropped; scans recorded; the profile.
// Whatever changed left as it was in base stays as onto has it.
func Merge(base, changed, onto *State) {
	onto.Files = mergeMap(base.Files, changed.Files, onto.Files)
	onto.Tracks = mergeMap(base.Tracks, changed.Tracks, onto.Tracks)

	for _, gone := range base.Past {
		if !slices.Contains(changed.Past, gone) {
			onto.Past = slices.DeleteFunc(onto.Past, func(a ArchiveEntry) bool { return a == gone })
		}
	}
	var added []ArchiveEntry
	for _, a := range changed.Past {
		if !slices.Contains(base.Past, a) {
			added = append(added, a)
		}
	}
	for _, a := range added {
		onto.Past = slices.DeleteFunc(onto.Past, func(b ArchiveEntry) bool { return b.Path == a.Path })
	}
	onto.Past = append(added, onto.Past...)
	if len(onto.Past) > maxArchive {
		onto.Past = onto.Past[:maxArchive]
	}

	for _, rec := range changed.History {
		seen := slices.ContainsFunc(base.History, func(b ScanRecord) bool { return b.Time.Equal(rec.Time) })
		if !seen {
			onto.History = append(onto.History, rec)
		}
	}
	if extra := len(onto.History) - maxHistory; extra > 0 {
		onto.History = slices.Clone(onto.History[extra:])
	}

	if changed.Profile != base.Profile {
		onto.Profile = changed.Profile
	}
}

// mergeMap applies the keys added, changed or removed between base and
// changed to onto, and returns it.
func mergeMap[V comparable](base, changed, onto map[string]V) map[string]V {
	if onto == nil {
		onto = map[string]V{}
	}
	for k, v := range changed {
		if old, ok := base[k]; !ok || old != v {
			onto[k] = v
		}
	}
	for k := range base {
		if _, ok := changed[k]; !ok {
			delete(onto, k)
		}
	}
	return onto
}
