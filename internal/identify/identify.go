// Package identify guesses what an unrecognized image file is, so the user
// only has to confirm a suggestion instead of searching the catalog.
//
// Nothing here changes anything: a guess is a suggestion with a reason the
// user can judge. Guesses come from evidence, best first:
//
//  1. a checksum the catalog publishes for that image;
//  2. the same file under another name, here or in the archive;
//  3. what the disc says about itself, and what the filename looks like.
package identify

import (
	"sort"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Guess is one catalog entry a file might be, with how sure isoshelf is and
// why.
type Guess struct {
	Entry *catalog.Entry
	// Version is the version this guess implies, when it can be worked out.
	Version string
	// Score runs from 0 to 100. Above 80 the evidence is hard: the same
	// checksum, or the same file under another name. Below that it is a
	// resemblance, which is exactly why a person confirms it.
	Score int
	// Reason is plain language: what led to this guess.
	Reason string
}

// Sure reports whether a guess rests on evidence rather than a resemblance.
func (g Guess) Sure() bool { return g.Score >= 80 }

const (
	// maxGuesses keeps the list short enough to read.
	maxGuesses = 6
	// minScore is the weakest resemblance worth showing.
	minScore = 30
)

// Suggest returns what the file at path might be, best guess first. The path
// is relative to the target, as in a scan. An empty result means nothing in
// the catalog resembles the file.
func Suggest(res *scan.Result, st *state.State, cat *catalog.Catalog, filePath string) []Guess {
	var file *scan.File
	for i := range res.Files {
		if res.Files[i].Path == filePath {
			file = &res.Files[i]
		}
	}
	if file == nil || cat == nil {
		return nil
	}
	var rec state.FileRecord
	if st != nil {
		rec = st.Files[filePath]
	}

	best := map[string]Guess{}
	add := func(g Guess) {
		if g.Entry == nil {
			return
		}
		if old, seen := best[g.Entry.ID]; !seen || g.Score > old.Score {
			if seen && g.Version == "" {
				g.Version = old.Version
			}
			best[g.Entry.ID] = g
		} else if old.Version == "" && g.Version != "" {
			old.Version = g.Version
			best[g.Entry.ID] = old
		}
	}

	// A name that fits more than one entry: the catalog can't choose, but
	// the user can.
	for _, m := range file.Matches {
		add(Guess{Entry: m.Entry, Version: m.Version, Score: 88,
			Reason: "its name fits this entry's pattern"})
	}
	for _, g := range fromHash(cat, rec) {
		add(g)
	}
	for _, g := range fromTwins(res, st, cat, file, rec) {
		add(g)
	}
	for _, g := range fromArchive(st, cat, rec) {
		add(g)
	}
	for _, g := range fromResemblance(cat, file) {
		add(g)
	}

	guesses := make([]Guess, 0, len(best))
	for _, g := range best {
		guesses = append(guesses, g)
	}
	sort.SliceStable(guesses, func(i, j int) bool {
		if guesses[i].Score != guesses[j].Score {
			return guesses[i].Score > guesses[j].Score
		}
		return guesses[i].Entry.Name < guesses[j].Entry.Name
	})
	if len(guesses) > maxGuesses {
		guesses = guesses[:maxGuesses]
	}
	return guesses
}
