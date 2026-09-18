package identify

import (
	"fmt"
	"path"
	"slices"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// fromHash matches the file's checksum against the known-good checksums in
// the catalog. Nothing is surer than this.
func fromHash(cat *catalog.Catalog, rec state.FileRecord) []Guess {
	if rec.SHA256 == "" {
		return nil
	}
	var out []Guess
	for i := range cat.Entries {
		if e := &cat.Entries[i]; slices.Contains(e.KnownHashes, rec.SHA256) {
			out = append(out, Guess{Entry: e, Score: 99,
				Reason: "its checksum is one published for this image"})
		}
	}
	return out
}

// fromTwins looks for the same image in the folder under another name. Two
// copies of one download have the same size and, inside, the same label and
// build time; if both have been hashed, that settles it.
func fromTwins(res *scan.Result, st *state.State, cat *catalog.Catalog, file *scan.File, rec state.FileRecord) []Guess {
	if st == nil {
		return nil
	}
	var out []Guess
	for _, other := range res.Files {
		if other.Path == file.Path {
			continue
		}
		twin := st.Files[other.Path]
		e := cat.Entry(twin.Entry)
		if e == nil {
			continue
		}
		switch {
		case rec.SHA256 != "" && rec.SHA256 == twin.SHA256:
			out = append(out, Guess{Entry: e, Version: twin.Version, Score: 97,
				Reason: fmt.Sprintf("it is byte for byte the same file as %s", path.Base(other.Path))})
		case file.Size == other.Size && sameBuild(file.Volume, other.Volume):
			out = append(out, Guess{Entry: e, Version: twin.Version, Score: 90,
				Reason: fmt.Sprintf("it is the same size as %s, and both discs say they were built on %s",
					path.Base(other.Path), file.Volume.Created.Format("2 January 2006"))})
		}
	}
	return out
}

// sameBuild reports whether two images say they came out of the same build:
// the same label, written at the same moment.
func sameBuild(a, b sniff.Volume) bool {
	return !a.Empty() && a.Label == b.Label && !a.Created.IsZero() && a.Created.Equal(b.Created)
}

// fromArchive recognizes a file isoshelf has seen in this folder before,
// under any name.
func fromArchive(st *state.State, cat *catalog.Catalog, rec state.FileRecord) []Guess {
	if st == nil || rec.SHA256 == "" {
		return nil
	}
	var out []Guess
	for _, past := range st.Archive() {
		e := cat.Entry(past.Entry)
		if e == nil || past.SHA256 != rec.SHA256 {
			continue
		}
		out = append(out, Guess{Entry: e, Version: past.Version, Score: 95,
			Reason: fmt.Sprintf("isoshelf saw this exact file here before, as %s", path.Base(past.Path))})
	}
	return out
}
