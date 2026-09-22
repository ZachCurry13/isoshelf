// Package inventory runs a scan or a check of one target from start to
// finish: load its state, scan, hash, check online, and save. The CLI and the
// web UI both use it.
package inventory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Stage is the part of a run in progress.
type Stage string

const (
	Scanning Stage = "scanning"
	Hashing  Stage = "hashing"
	Checking Stage = "checking"
	Saving   Stage = "saving"
)

// Progress describes where a run is.
type Progress struct {
	Stage Stage
	// File is the file being hashed.
	File string
	// Done and Total count bytes of File while hashing, and entries while
	// checking.
	Done, Total int64
	// Item and Items count the files being hashed: "3 of 7".
	Item, Items int
}

// Options control a run.
type Options struct {
	Target string
	// Profile, if set, replaces the profile saved in the target's state.
	Profile scan.Profile
	// Online checks sources for updates; Client must then be set.
	Online bool
	Client *remote.Client
	// Memory, if set, is what isoshelf found last time it asked: an entry it
	// already has a fresh answer for isn't asked about again.
	Memory check.Memory
	// NoHash skips hashing fixed-name images.
	NoHash  bool
	Catalog *catalog.Catalog
	// Dirs locate the app: its folder is never scanned, and outside portable
	// mode the state mirror goes to its config folder.
	Dirs appdir.Dirs
	// Records is where this folder's records are kept. The zero value is the
	// default: inside the folder itself.
	Records state.Home
	// HashAll hashes every recognized image rather than only those whose
	// filename never changes. Sharing needs it: another isoshelf asks for a
	// file by its hash, so a file with no hash can't be offered.
	HashAll bool
	// Now defaults to time.Now.
	Now func() time.Time
	// Progress, if not nil, is called as the run moves along.
	Progress func(Progress)
	// Interim, if not nil, is handed the result as soon as the folder has
	// been listed, before hashing, which is the slow part. The same Result
	// is filled in further and returned at the end.
	Interim func(*Result)
}

// Result is the outcome of a run.
type Result struct {
	Report *check.Report
	State  *state.State
	// Scan is what the folder held, kept so the caller can look at a file
	// again without reading the disk.
	Scan *scan.Result
	// Warnings are problems that didn't stop the run, such as a state that
	// couldn't be saved to a read-only share.
	Warnings []string
}

// Run scans the target and, if asked, checks it online. The state is saved
// even when ctx is cancelled partway, in which case Run returns the result so
// far together with ctx.Err().
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Online && opts.Client == nil {
		return nil, errors.New("inventory: an online check needs a client")
	}
	progress := func(p Progress) {
		if opts.Progress != nil {
			opts.Progress(p)
		}
	}
	target, err := filepath.Abs(opts.Target)
	if err != nil {
		return nil, err
	}

	st, err := opts.Records.Load(target)
	if err != nil {
		return nil, err
	}
	// The copy to measure this scan's own changes against. A download can
	// finish while the scan reads the folder, and its file must not be lost
	// when the scan saves (see SaveOnto).
	base := st.Clone()
	if opts.Profile != "" {
		st.Profile = opts.Profile
	}
	progress(Progress{Stage: Scanning})
	var skip []string
	if opts.Dirs.App != "" {
		skip = append(skip, opts.Dirs.App)
	}
	res, err := scan.Scan(ctx, target, opts.Catalog, scan.Options{Profile: st.Profile, Skip: skip})
	if err != nil {
		return nil, err
	}
	st.RecordScan(res, opts.Now())
	out := &Result{State: st, Scan: res}

	// What the folder holds is known now. Hashing comes next and can take
	// minutes on a USB drive, so the caller is handed the list first and the
	// numbers fill in behind it.
	out.Report = check.Offline(res, st, opts.Catalog)
	if opts.Interim != nil {
		opts.Interim(out)
	}

	if !opts.NoHash {
		files := st.NeedsHash(res, opts.Catalog, opts.HashAll)
		for i := range files {
			err := st.HashFiles(ctx, target, files[i:i+1], func(f scan.File, done int64) {
				progress(Progress{
					Stage: Hashing, File: f.Path, Done: done, Total: f.Size,
					Item: i + 1, Items: len(files),
				})
			})
			if err != nil && ctx.Err() == nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("couldn't hash %v", err))
			}
		}
		// Hashes decide whether a fixed-name image is out of date, so the
		// report is built again now that they are known.
		out.Report = check.Offline(res, st, opts.Catalog)
	}
	if opts.Online && ctx.Err() == nil {
		progress(Progress{Stage: Checking})
		out.Report.Online(ctx, opts.Client, st, opts.Memory, func(done, total int) {
			progress(Progress{Stage: Checking, Done: int64(done), Total: int64(total)})
		})
	}

	progress(Progress{Stage: Saving})
	// Only what this scan learned goes onto the records as they are now, so
	// that a download which placed a file meanwhile keeps its own.
	saved, err := opts.Records.SaveOnto(st, target, base)
	if err != nil {
		where, dirErr := opts.Records.Dir(target)
		if dirErr != nil {
			where = string(opts.Records)
		}
		out.Warnings = append(out.Warnings, fmt.Sprintf("couldn't save what was learned to %s: %v", where, err))
		saved = st
	}
	out.State = saved
	if !opts.Dirs.Portable && opts.Dirs.Config != "" {
		if err := saved.SaveMirror(opts.Dirs.Config, target, opts.Now()); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("couldn't save a copy of the history: %v", err))
		}
	}
	return out, ctx.Err()
}
