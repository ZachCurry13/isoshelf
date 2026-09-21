// Package check turns a scan into a status report: which images are up to
// date, which have updates, which are end of life, and which need attention.
// The CLI and the web UI both show these reports.
package check

import (
	"cmp"
	"context"
	"errors"
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
	// Added is when the file arrived in the folder, as well as that can be
	// known; zero when it can't.
	Added time.Time
	// Placed is true when isoshelf downloaded and placed this file, rather
	// than the file just turning up in the folder.
	Placed bool
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
	// CheckedAt is when the oldest answer the report rests on was given.
	// A check that reused yesterday's answers says yesterday.
	CheckedAt time.Time
}

// Offline builds a report from a scan and the target's state, without the
// network. Call it after st.RecordScan.
func Offline(res *scan.Result, st *state.State, cat *catalog.Catalog) *Report {
	r := &Report{Target: res.Root, Profile: res.Profile, Trash: res.Trash, Problems: res.Problems}
	found := map[string]bool{}
	for _, f := range res.Files {
		it := Item{Path: f.Path, Size: f.Size, Kind: f.Kind, ModTime: f.ModTime}
		rec := st.Files[f.Path]
		it.Added = addedAt(f, rec)
		it.Placed = !rec.PlacedAt.IsZero()
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

// Memory is what isoshelf found the last time it asked. Online asks it
// before the network, and tells it every fresh answer. A nil Memory means
// every entry is asked about.
type Memory interface {
	// Recall returns an answer worth reusing, and whether there was one.
	Recall(entry string) (*source.Release, *resolve.Artifact, bool)
	// Remember keeps an answer that has just come back.
	Remember(entry string, rel *source.Release, art *resolve.Artifact)
	// Asked says when an entry's answer is from.
	Asked(entry string) time.Time
}

// Online asks each recognized entry's source for its latest release and fills
// in the statuses. Entries are checked a few at a time; a failure affects only
// that entry's items. An entry mem already has a fresh answer for is not asked
// again, which is what makes opening the page cost nothing. progress, if not
// nil, is called after each entry with how many of them are done.
func (r *Report) Online(ctx context.Context, client *remote.Client, st *state.State, mem Memory, progress func(done, total int)) {
	byEntry := map[string][]int{}
	for i, it := range r.Items {
		if it.Entry != nil && it.Entry.Source.Type != catalog.SourceManual {
			byEntry[it.Entry.ID] = append(byEntry[it.Entry.ID], i)
		}
	}

	now := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex
	limit := make(chan struct{}, 4)
	done := 0
	for _, indexes := range byEntry {
		wg.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			e := r.Items[indexes[0]].Entry
			rel, art, remembered := recall(mem, e.ID)
			var err error
			if !remembered {
				rel, art, err = latest(ctx, client, e)
				// Only answers worth reusing are kept; a site that was down
				// is asked again next time rather than looking like bad news
				// until tomorrow.
				if err == nil && mem != nil {
					mem.Remember(e.ID, rel, art)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			for _, i := range indexes {
				it := &r.Items[i]
				decide(it, rel, art, err, st.Files[it.Path].SHA256)
			}
			// The report is only as fresh as its oldest answer, and the page
			// says so rather than claiming it all just happened.
			if err == nil {
				if asked := askedAt(mem, e.ID, now); r.CheckedAt.IsZero() || asked.Before(r.CheckedAt) {
					r.CheckedAt = asked
				}
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

// recall asks what isoshelf already knows, if anything.
func recall(mem Memory, entry string) (*source.Release, *resolve.Artifact, bool) {
	if mem == nil {
		return nil, nil, false
	}
	return mem.Recall(entry)
}

// askedAt is when an entry's answer is from, falling back to now for an
// answer nothing is remembering.
func askedAt(mem Memory, entry string, now time.Time) time.Time {
	if mem == nil {
		return now
	}
	if at := mem.Asked(entry); !at.IsZero() {
		return at
	}
	return now
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

// addedAt works out when a file arrived in the folder: the system's own
// record where it keeps one (Windows, and network shares), else when
// isoshelf put it there, else when a scan first found it. A file copied in
// keeps its old modification time, so that is never used.
func addedAt(f scan.File, rec state.FileRecord) time.Time {
	for _, t := range []time.Time{f.Created, rec.PlacedAt, rec.FirstSeen} {
		if !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}
