// Package check turns a scan into a status report: which images are up to
// date, which have updates, which are end of life, and which need attention.
// The CLI and the web UI both show these reports.
package check

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
	"github.com/ZachCurry13/isoshelf/internal/version"
)

// Status is the state of one image.
type Status string

const (
	UpToDate         Status = "up to date"
	UpdateAvailable  Status = "update available"
	EOL              Status = "EOL"
	Missing          Status = "missing"
	Manual           Status = "manual"
	ChecksumMismatch Status = "checksum mismatch"
	NotBootable      Status = "not bootable"
	Unrecognized     Status = "unrecognized"
	CheckFailed      Status = "check failed"
	// Unknown means the check ran but couldn't decide, for example because
	// a fixed-name image hasn't been hashed yet.
	Unknown Status = "unknown"
	// NotChecked is the status of recognized images before an online check.
	NotChecked Status = "not checked"
)

// order sorts statuses so the ones that need attention come first.
var order = []Status{
	ChecksumMismatch, UpdateAvailable, EOL, NotBootable, Missing, Unrecognized,
	CheckFailed, Unknown, NotChecked, Manual, UpToDate,
}

// Item is one row of a report: a file, or a usual-set entry that's missing.
type Item struct {
	// Path is relative to the target, with / separators. Empty for missing
	// entries.
	Path string
	Size int64
	Kind sniff.Kind
	// Entry is the catalog entry the file belongs to, or nil.
	Entry *catalog.Entry
	// Version is the version of the file on the target.
	Version string
	Status  Status
	// EOL is set when the file's own release cycle has reached end of life,
	// whatever the status.
	EOL bool
	// Latest is the newest version, and LatestFile its filename, when known.
	Latest     string
	LatestFile string
	// Assigned marks a file the user identified by hand, rather than one the
	// catalog matched by name.
	Assigned bool
	// Older marks a file of an image the folder holds a newer copy of.
	Older bool
	// Note explains the status: an error, or "older copy" and so on.
	Note string
	// Release is a page about the newest release, when the source has one.
	Release string
	// ModTime is when the file last changed.
	ModTime time.Time
}

// Report is the state of every image on a target.
type Report struct {
	Target   string
	Profile  scan.Profile
	Items    []Item
	Trash    []scan.Trash
	Problems []scan.Problem
	// Checked is set once the online check has run.
	Checked bool
}

// Offline builds a report from a scan and the target's state, without the
// network. Call it after st.RecordScan.
func Offline(res *scan.Result, st *state.State, cat *catalog.Catalog) *Report {
	r := &Report{Target: res.Root, Profile: res.Profile, Trash: res.Trash, Problems: res.Problems}
	found := map[string]bool{}
	for _, f := range res.Files {
		it := Item{Path: f.Path, Size: f.Size, Kind: f.Kind, ModTime: f.ModTime}
		rec := st.Files[f.Path]
		if e := cat.Entry(rec.Entry); e != nil {
			it.Entry, it.Version, it.Assigned = e, rec.Version, rec.Assigned
			found[e.ID] = true
		}
		switch {
		case it.Entry == nil:
			it.Status = Unrecognized
			if len(f.Matches) > 1 {
				var ids []string
				for _, m := range f.Matches {
					ids = append(ids, m.Entry.ID)
				}
				it.Note = "matches several catalog entries: " + strings.Join(ids, ", ")
			}
		case !f.Bootable && res.Profile.Boots():
			it.Status = NotBootable
			it.Note = notBootableNote(it.Entry, res.Profile)
		case it.Entry.Source.Type == catalog.SourceManual:
			it.Status = Manual
			if rec.SHA256 != "" && slices.Contains(it.Entry.KnownHashes, rec.SHA256) {
				it.Note = "matches a known-good hash"
			}
		default:
			it.Status = NotChecked
		}
		r.Items = append(r.Items, it)
	}
	for _, id := range st.Missing() {
		if e := cat.Entry(id); e != nil && !found[id] {
			r.Items = append(r.Items, Item{Entry: e, Status: Missing})
		}
	}
	markOlderCopies(r.Items)
	r.sort()
	return r
}

func notBootableNote(e *catalog.Entry, profile scan.Profile) string {
	exts := strings.Join(profile.Extensions(), " ")
	if e.Fixup != "" {
		return fmt.Sprintf("%s lists only %s; Make bootable can fix this (%s)", profile, exts, e.Fixup)
	}
	return fmt.Sprintf("%s lists only %s", profile, exts)
}

// Online asks each recognized entry's source for its latest release and fills
// in the statuses. Entries are checked a few at a time; a failure affects only
// that entry's items. progress, if not nil, is called after each entry with
// how many of them are done.
func (r *Report) Online(ctx context.Context, client *remote.Client, st *state.State, progress func(done, total int)) {
	byEntry := map[string][]int{}
	for i, it := range r.Items {
		if it.Entry != nil && it.Entry.Source.Type != catalog.SourceManual {
			byEntry[it.Entry.ID] = append(byEntry[it.Entry.ID], i)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	limit := make(chan struct{}, 4)
	done := 0
	for _, indexes := range byEntry {
		wg.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			e := r.Items[indexes[0]].Entry
			rel, art, err := latest(ctx, client, e)
			mu.Lock()
			defer mu.Unlock()
			for _, i := range indexes {
				it := &r.Items[i]
				decide(it, rel, art, err, st.Files[it.Path].SHA256)
			}
			done++
			if progress != nil {
				progress(done, len(byEntry))
			}
		})
	}
	wg.Wait()
	markOlderCopies(r.Items)
	r.Checked = true
	r.sort()
}

// latest finds an entry's latest release and, if it has one, its file.
func latest(ctx context.Context, client *remote.Client, e *catalog.Entry) (*source.Release, *resolve.Artifact, error) {
	rel, err := source.Latest(ctx, client, e)
	if err != nil {
		return nil, nil, err
	}
	art, err := resolve.Resolve(ctx, client, e, rel)
	if errors.Is(err, resolve.ErrNoArtifact) {
		return rel, nil, nil
	}
	return rel, art, err
}

// decide sets an item's status from the check result. recorded is the
// SHA-256 recorded for the file, or empty.
func decide(it *Item, rel *source.Release, art *resolve.Artifact, err error, recorded string) {
	if err != nil {
		switch it.Status {
		case NotBootable: // keep the more useful note
		case Missing:
			it.Note = err.Error()
		default:
			it.Status, it.Note = CheckFailed, err.Error()
		}
		return
	}
	e := it.Entry
	it.Latest, it.Release = rel.Version, rel.URL
	if art != nil {
		it.LatestFile = art.Filename
		if v, ok := e.MatchName(art.Filename); ok && v != "" {
			it.Latest = v
		}
	}
	if it.Status == Missing {
		return
	}
	it.EOL = fileCycleEOL(e, rel, it.Version)

	status, note := compare(it, e, art, recorded)
	switch {
	case it.Status == NotBootable:
		// Keep the status; the latest version is still worth showing.
	case status == UpToDate && it.EOL:
		it.Status = EOL
	default:
		it.Status, it.Note = status, note
	}
}

func compare(it *Item, e *catalog.Entry, art *resolve.Artifact, recorded string) (Status, string) {
	if e.FixedName {
		switch {
		case art == nil || art.Checksum == nil:
			return Unknown, "no published checksum to compare with"
		case art.Checksum.Algorithm != verify.SHA256:
			return Unknown, fmt.Sprintf("the published checksum is %s; only SHA-256 can be compared", art.Checksum.Algorithm)
		case recorded == "":
			return Unknown, "not hashed yet"
		case recorded == art.Checksum.Hex:
			return UpToDate, ""
		}
		return UpdateAvailable, "the published checksum changed"
	}

	if art != nil && strings.EqualFold(art.Filename, path.Base(it.Path)) {
		if recorded != "" && art.Checksum != nil && art.Checksum.Algorithm == verify.SHA256 && recorded != art.Checksum.Hex {
			return ChecksumMismatch, "the file doesn't match its published SHA-256"
		}
		return UpToDate, ""
	}
	if it.Version == "" {
		return Unknown, "the filename has no version to compare"
	}
	if version.Compare(it.Latest, it.Version) > 0 {
		return UpdateAvailable, ""
	}
	return UpToDate, ""
}

// fileCycleEOL reports whether the endoflife.date cycle the file belongs to
// has reached end of life. A pinned channel is the file's cycle; otherwise
// the cycle is found from the file's version.
func fileCycleEOL(e *catalog.Entry, rel *source.Release, v string) bool {
	if e.Source.Type != catalog.SourceEndOfLife {
		return false
	}
	var c *source.Cycle
	if ch := e.Source.Channel; ch != "latest" && ch != "lts" {
		c = rel.CycleFor(rel.Cycle)
	} else {
		c = rel.CycleFor(v)
	}
	return c != nil && c.EOL
}

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

func (r *Report) sort() {
	slices.SortStableFunc(r.Items, func(a, b Item) int {
		return cmp.Or(
			cmp.Compare(slices.Index(order, a.Status), slices.Index(order, b.Status)),
			strings.Compare(a.Name(), b.Name()),
			strings.Compare(a.Path, b.Path),
		)
	})
}

// Name is the item's track name, or its filename when unrecognized.
func (it Item) Name() string {
	if it.Entry != nil {
		return it.Entry.Name
	}
	return path.Base(it.Path)
}

// Counts returns how many items have each status.
func (r *Report) Counts() map[Status]int {
	counts := map[Status]int{}
	for _, it := range r.Items {
		counts[it.Status]++
	}
	return counts
}

// Order returns statuses in the order reports list them.
func Order() []Status {
	return slices.Clone(order)
}
