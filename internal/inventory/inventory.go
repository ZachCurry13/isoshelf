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
}

// Options control a run.
type Options struct {
	Target string
	// Profile, if set, replaces the profile saved in the target's state.
	Profile scan.Profile
	// Online checks sources for updates; Client must then be set.
	Online bool
	Client *remote.Client
	// NoHash skips hashing fixed-name images.
	NoHash  bool
	Catalog *catalog.Catalog
	// Dirs locate the app: its folder is never scanned, and outside portable
	// mode the state mirror goes to its config folder.
	Dirs appdir.Dirs
	// Now defaults to time.Now.
	Now func() time.Time
	// Progress, if not nil, is called as the run moves along.
	Progress func(Progress)
}

// Result is the outcome of a run.
type Result struct {
	Report *check.Report
	State  *state.State
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

	st, err := state.Load(target)
	if err != nil {
		return nil, err
	}
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
	out := &Result{State: st}

	if !opts.NoHash {
		files := st.NeedsHash(res, opts.Catalog)
		for i := range files {
			err := st.HashFiles(ctx, target, files[i:i+1], func(f scan.File, done int64) {
				progress(Progress{Stage: Hashing, File: f.Path, Done: done, Total: f.Size})
			})
			if err != nil && ctx.Err() == nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("couldn't hash %v", err))
			}
		}
	}

	out.Report = check.Offline(res, st, opts.Catalog)
	if opts.Online && ctx.Err() == nil {
		progress(Progress{Stage: Checking})
		out.Report.Online(ctx, opts.Client, st, func(done, total int) {
			progress(Progress{Stage: Checking, Done: int64(done), Total: int64(total)})
		})
	}

	progress(Progress{Stage: Saving})
	if err := st.Save(target); err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("couldn't save what was learned to %s: %v", filepath.Join(target, state.DirName), err))
	}
	if !opts.Dirs.Portable && opts.Dirs.Config != "" {
		if err := st.SaveMirror(opts.Dirs.Config, target, opts.Now()); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("couldn't save a copy of the history: %v", err))
		}
	}
	return out, ctx.Err()
}
