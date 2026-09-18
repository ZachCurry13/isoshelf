package catalog

// Match is an entry whose pattern matched a filename.
type Match struct {
	Entry *Entry
	// Version is the captured version, or empty when the filename has none.
	Version string
}

// Entry returns the entry with the given id, or nil.
func (c *Catalog) Entry(id string) *Entry {
	return c.byID[id]
}

// Match returns every entry whose pattern matches filename. In a valid
// catalog this is usually zero or one entry.
func (c *Catalog) Match(filename string) []Match {
	var out []Match
	for i := range c.Entries {
		if version, ok := c.Entries[i].MatchName(filename); ok {
			out = append(out, Match{Entry: &c.Entries[i], Version: version})
		}
	}
	return out
}

// MatchName reports whether filename belongs to this entry, and returns the
// version captured from it.
func (e *Entry) MatchName(filename string) (version string, ok bool) {
	if e.match == nil {
		return "", false
	}
	m := e.match.FindStringSubmatch(filename)
	if m == nil {
		return "", false
	}
	if i := e.match.SubexpIndex("version"); i >= 0 {
		version = m[i]
	}
	return version, true
}
