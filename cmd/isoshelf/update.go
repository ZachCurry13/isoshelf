package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
	inv "github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/lastcheck"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// isoshelf update (v0.8.5, #1): check the folder, then download every update
// isoshelf can - or only the ones named with --only - verified exactly as the
// page does, and deal with each old file the way a flag says. There is no
// default for that: the page's answer lives in Settings, and a command line
// in a cron job should say what it means rather than inherit it.

func updateImages(ctx context.Context, e *env, opts options) error {
	removal, err := removalFlag(opts)
	if err != nil {
		return err
	}
	dirs, err := findDirs(e)
	if err != nil {
		return err
	}
	target := opts.folder
	if target == "" {
		if !dirs.Portable {
			return errors.New(`which folder? For example: isoshelf update --keep E:\`)
		}
		target = dirs.DefaultTarget
	}
	cat, _, err := loadCatalog(dirs, opts.catalog, e.stderr)
	if err != nil {
		return err
	}
	cat = withOwnImages(cat, dirs, e.stderr)
	client := remote.New(version)
	client.HTTP = e.http
	client.GitHubToken = e.getenv("GITHUB_TOKEN")
	answers := lastcheck.Load(dirs.Config)
	answers.Now = e.now
	records := state.Home(settings.Load(dirs.Config).RecordsHome(target, dirs.Config))

	res, err := inv.Run(ctx, inv.Options{
		Target: target, Profile: scan.Profile(opts.profile), Online: true, Memory: answers.Asking(),
		Client: client, Catalog: cat, Dirs: dirs, Records: records, Now: e.now,
		Progress: func(p inv.Progress) { progress(e, opts, p) },
	})
	progress(e, opts, inv.Progress{})
	answers.Save() // best effort
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(e.stderr, "isoshelf:", w)
	}

	todo, err := chooseUpdates(res, opts.only, e)
	if err != nil {
		return err
	}
	if len(todo) == 0 {
		fmt.Fprintln(e.stdout, "Nothing to update.")
		return nil
	}
	if opts.dryRun {
		var total int64
		for _, it := range todo {
			total += it.Entry.Size
			fmt.Fprintf(e.stdout, "Would update %s: %s -> %s%s\n", it.Entry.Name, orDash(it.Version), it.Latest, aboutSize(it.Entry.Size))
		}
		fmt.Fprintf(e.stdout, "%d to download%s. Nothing was changed.\n", len(todo), aboutSize(total))
		return nil
	}

	failed := 0
	for _, it := range todo {
		if err := updateOne(ctx, e, opts, target, records, cat, res.Report, it, removal); err != nil {
			fmt.Fprintf(e.stderr, "isoshelf: %s: %v\n", it.Entry.Name, err)
			failed++
			if ctx.Err() != nil {
				break
			}
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d updates failed; nothing was replaced for those", failed, len(todo))
	}
	return nil
}

// removalFlag is what happens to the old file, which has to be said.
func removalFlag(opts options) (update.Removal, error) {
	var chosen []update.Removal
	if opts.keep {
		chosen = append(chosen, update.Keep)
	}
	if opts.moveAside {
		chosen = append(chosen, update.MoveAside)
	}
	if opts.deleteOld {
		chosen = append(chosen, update.DeleteNow)
	}
	switch {
	case len(chosen) == 1:
		return chosen[0], nil
	case len(chosen) > 1:
		return "", errors.New("say one of --keep, --move-aside or --delete, not several")
	case opts.dryRun:
		return update.Keep, nil
	}
	return "", errors.New("say what happens to each old file: --keep, --move-aside (to the archive) or --delete")
}

// chooseUpdates is every update isoshelf can download, less dismissed ones,
// or exactly the entries named - which must each have one.
func chooseUpdates(res *inv.Result, only []string, e *env) ([]check.Item, error) {
	var out []check.Item
	for _, it := range res.Report.Items {
		if it.Status != check.UpdateAvailable || it.Entry == nil || it.Entry.Updates() != catalog.UpdatesDownload {
			continue
		}
		if len(only) > 0 {
			if slices.Contains(only, it.Entry.ID) && !slices.ContainsFunc(out, sameEntry(it)) {
				out = append(out, it)
			}
			continue
		}
		// Dismissed on the page means not now; naming it with --only is asking.
		if res.State.Track(it.Entry.ID).Dismissed(e.now()) || slices.ContainsFunc(out, sameEntry(it)) {
			continue
		}
		out = append(out, it)
	}
	for _, id := range only {
		if !slices.ContainsFunc(out, func(it check.Item) bool { return it.Entry.ID == id }) {
			return nil, fmt.Errorf("%s has no update isoshelf can download here", id)
		}
	}
	return out, nil
}

func sameEntry(it check.Item) func(check.Item) bool {
	return func(o check.Item) bool { return o.Entry.ID == it.Entry.ID }
}

// updateOne downloads one image and saves what it changed onto the records
// as they are now.
func updateOne(ctx context.Context, e *env, opts options, target string, records state.Home, cat *catalog.Catalog, report *check.Report, it check.Item, removal update.Removal) error {
	st, err := records.Load(target)
	if err != nil {
		return err
	}
	base := st.Clone()
	var old []string
	for _, other := range report.Items {
		if other.Path != "" && other.Entry != nil && other.Entry.ID == it.Entry.ID {
			old = append(old, other.Path)
		}
	}
	var pinned []string
	for _, p := range st.Pinned() {
		if slices.Contains(old, p) {
			pinned = append(pinned, p)
		}
	}
	client := remote.New(version)
	client.HTTP = e.http
	fetcher := fetch.New(version)
	fetcher.HTTP = e.http
	fetcher.GitHubToken = e.getenv("GITHUB_TOKEN")
	res, err := update.Run(ctx, update.Options{
		Target: target, Entry: cat.Entry(it.Entry.ID), Client: client, Fetcher: fetcher,
		State: st, Old: old, Pinned: pinned, Removal: removal, Now: e.now,
		Progress: func(p fetch.Progress) {
			if e.interactive && p.Total > 0 {
				fmt.Fprintf(e.stderr, "\r%-78.78s\r%s: %d%%", "", it.Entry.Name, p.Done*100/p.Total)
			}
		},
	})
	if e.interactive {
		fmt.Fprintf(e.stderr, "\r%-78.78s\r", "")
	}
	if _, saveErr := records.SaveOnto(st, target, base); err == nil {
		err = saveErr
	}
	if err != nil {
		return err
	}
	line := fmt.Sprintf("Updated %s to %s: %s", it.Entry.Name, res.Version, res.File)
	if len(res.Removed) > 0 {
		line += "; old: " + strings.Join(res.Removed, ", ")
	}
	if len(res.Kept) > 0 {
		line += "; kept: " + strings.Join(res.Kept, ", ")
	}
	if !res.Verified {
		line += " (unverified: the project publishes no checksum, so nothing old was replaced)"
	}
	fmt.Fprintln(e.stdout, line)
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func aboutSize(n int64) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(", about %.1f GB", float64(n)/(1<<30))
}
