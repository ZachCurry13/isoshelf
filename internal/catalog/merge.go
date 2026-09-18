package catalog

import (
	"errors"
	"slices"
)

// Newer reports whether c is at least as new as other. A catalog without a
// revision counts as the oldest there is.
func (c *Catalog) Newer(other *Catalog) bool {
	if c == nil {
		return false
	}
	if other == nil {
		return true
	}
	return c.Revision >= other.Revision
}

// Merge returns a catalog holding base's entries plus extra's: an entry whose
// id is already in base replaces it, and the rest are added. The result is
// validated as a whole, so an added entry that clashes with a built-in one is
// an error rather than a surprise later. Neither input is changed.
//
// Merging catalogs that declare signing keys isn't supported: the key files
// are named relative to their own catalog, and there is nowhere to resolve
// both from.
func Merge(base, extra *Catalog) (*Catalog, error) {
	if base == nil {
		return extra, nil
	}
	if extra == nil || len(extra.Entries) == 0 {
		return base, nil
	}
	if len(base.Keys) > 0 || len(extra.Keys) > 0 {
		return nil, errors.New("catalog: entries of your own can't be merged into a catalog with signing keys")
	}
	merged := &Catalog{Schema: SchemaVersion, Entries: slices.Clone(base.Entries)}
	for _, add := range extra.Entries {
		if i := slices.IndexFunc(merged.Entries, func(e Entry) bool { return e.ID == add.ID }); i >= 0 {
			merged.Entries[i] = add
			continue
		}
		merged.Entries = append(merged.Entries, add)
	}
	if err := merged.validate(nil, "catalog"); err != nil {
		return nil, err
	}
	return merged, nil
}
