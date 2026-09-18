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
	"fmt"
	"math"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
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

// fromResemblance scores every entry on how much of its name turns up in the
// filename and in what the disc calls itself. Rare words count for more than
// common ones: "qubes" says far more than "linux".
func fromResemblance(cat *catalog.Catalog, file *scan.File) []Guess {
	weights := tokenWeights(cat)
	name := strings.TrimSuffix(file.Name(), path.Ext(file.Name()))
	says := file.Volume.Says()
	nameTokens, saysTokens := tokens(name), tokens(says)
	fileArch := firstArch(name, says)

	var out []Guess
	for i := range cat.Entries {
		e := &cat.Entries[i]
		want := entryTokens(e, weights)
		if len(want) == 0 {
			continue
		}
		byName := 70 * cover(want, nameTokens)
		bySays := 65 * cover(want, saysTokens)
		score := math.Max(byName, bySays) + math.Min(byName, bySays)/3
		if score < 1 {
			continue
		}
		switch {
		case fileArch == "" || e.Arch == "multi":
		case fileArch == e.Arch:
			score += 8
		default:
			score -= 40 // a 32-bit file is never the 64-bit track
		}
		if !sameShape(e, file.Name()) {
			score -= 25 // this track ships another kind of file entirely
		}
		g := Guess{Entry: e, Score: int(math.Min(score, 85))}
		if g.Score < minScore {
			continue
		}
		switch {
		case byName >= 1 && bySays >= 1:
			g.Reason = fmt.Sprintf("the file name and the disc label (%q) both point to it", file.Volume.Label)
		case bySays >= 1:
			g.Reason = fmt.Sprintf("the disc calls itself %q", strings.TrimSpace(file.Volume.Label))
		default:
			g.Reason = "the file name looks like it"
		}
		g.Version = firstVersion(says, name)
		out = append(out, g)
	}
	return out
}

// sameShape reports whether a file's extension is one this entry's real
// filenames use. Ubuntu Core ships .img.xz, so an .iso is something else,
// however much the names agree. An entry without samples matches anything.
func sameShape(e *catalog.Entry, filename string) bool {
	ext := strings.ToLower(path.Ext(filename))
	if len(e.Samples) == 0 || ext == "" {
		return true
	}
	for _, s := range e.Samples {
		if strings.ToLower(path.Ext(s)) == ext {
			return true
		}
	}
	return false
}

// cover returns how much of an entry's name, by weight, turns up in have.
func cover(want map[string]float64, have []string) float64 {
	var total, found float64
	for tok, w := range want {
		total += w
		if best := bestMatch(tok, have); best > 0 {
			found += w * best
		}
	}
	if total == 0 {
		return 0
	}
	return found / total
}

// bestMatch reports how well tok is present in have: 1 for the same word,
// 0.7 for an abbreviation ("win" for "windows"), 0 for absent.
func bestMatch(tok string, have []string) float64 {
	best := 0.0
	for _, h := range have {
		switch {
		case h == tok:
			return 1
		case len(tok) >= 3 && len(h) >= 3 && (strings.HasPrefix(h, tok) || strings.HasPrefix(tok, h)):
			best = math.Max(best, 0.7)
		}
	}
	return best
}

// entryTokens returns the words that identify an entry, with their weights.
func entryTokens(e *catalog.Entry, weights map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for _, tok := range tokens(e.Name + " " + strings.ReplaceAll(e.ID, "-", " ") + " " + e.Family) {
		if w := weights[tok]; w > 0 {
			out[tok] = w
		}
	}
	return out
}

// tokenWeights gives every word in the catalog a weight: the fewer entries
// use a word, the more it says about which entry a file belongs to.
func tokenWeights(cat *catalog.Catalog) map[string]float64 {
	in := map[string]int{}
	for i := range cat.Entries {
		e := &cat.Entries[i]
		for _, tok := range unique(tokens(e.Name + " " + strings.ReplaceAll(e.ID, "-", " ") + " " + e.Family)) {
			in[tok]++
		}
	}
	weights := make(map[string]float64, len(in))
	n := float64(len(cat.Entries))
	for tok, used := range in {
		weights[tok] = math.Log(1 + n/float64(used))
	}
	return weights
}

var (
	wordPattern = regexp.MustCompile(`[a-z]+|[0-9]+`)
	// Words that say nothing about which image a file is: they are either
	// everywhere or describe the medium. Architecture is handled separately.
	noise = newSet(
		"iso", "img", "wim", "vhd", "vhdx", "efi",
		"cd", "dvd", "disc", "image", "live",
		"install", "installer", "current", "latest",
		"stable", "release", "final", "full", "free",
		"en", "us", "english", "x", "v", "r", "bit",
		"x86", "x64", "i386", "i686", "x32", "amd", "amd64",
		"arm64", "aarch64", "armhf", "multi", "32", "64",
	)
)

func newSet(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return set
}

// tokens splits text into the lowercase words and numbers it is made of, and
// drops the ones that say nothing.
func tokens(s string) []string {
	var out []string
	for _, tok := range wordPattern.FindAllString(strings.ToLower(s), -1) {
		if noise[tok] {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func unique(s []string) []string {
	slices.Sort(s)
	return slices.Compact(s)
}

// archPatterns read an architecture out of a filename or a disc label, most
// specific first.
var archPatterns = []struct {
	re   *regexp.Regexp
	arch string
}{
	{regexp.MustCompile(`(?i)arm64|aarch64`), "arm64"},
	{regexp.MustCompile(`(?i)armhf|armv7|\barm\b`), "arm"},
	{regexp.MustCompile(`(?i)x86[-_]?64|amd64|\bx64\b|64[-_ ]?bit`), "x86_64"},
	{regexp.MustCompile(`(?i)i[3-6]86|\bx32\b|\bx86\b|32[-_ ]?bit`), "x86"},
}

// firstArch returns the architecture the given texts name, or "".
func firstArch(texts ...string) string {
	for _, text := range texts {
		for _, p := range archPatterns {
			if p.re.MatchString(text) {
				return p.arch
			}
		}
	}
	return ""
}

// A version worth suggesting has a dot ("22.04.3") or is a date-like build
// number ("20231102"). A lone digit, as in "Kali Linux amd64 1", is a disc
// number as often as a version, so it is left out.
var versionPattern = regexp.MustCompile(`\d+\.\d+(?:\.\d+)*|\b\d{6,8}\b`)

// firstVersion returns the first version-looking number in the given texts.
func firstVersion(texts ...string) string {
	for _, text := range texts {
		if v := versionPattern.FindString(text); v != "" {
			return v
		}
	}
	return ""
}
