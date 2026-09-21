# Status

## Where things stand (2026-09-18)

- **Released:** v0.3.3 (fixes and tidying; v0.3.2 checking by itself, v0.3.1
  Settings, v0.3.0 the redesign). Scans a folder, works out each image, checks for
  updates online, downloads and verifies (resumable, queued one at a time,
  reorderable), replaces or archives old files, identifies unknown files, and
  keeps a catalog that updates itself from this repository. While downloads
  run, removing, archiving and identifying still work (the state file is
  merged, never overwritten). Web page and CLI; Windows and Linux builds.
- **Catalog:** 86 entries (60 downloadable, 9 update-check only, 17 link only),
  revision `2026091804`. Checked live every Monday by
  `.github/workflows/catalog-check.yml` and on every catalog pull request; a
  weekly Claude routine ("isoshelf weekly catalog", Mondays 18:00 UTC, 1 pm
  Chicago, just after the maintainer's weekly usage resets;
  https://claude.ai/code/routines/trig_01UzAxtUxF8jd4RwavQXAYdL) works through
  `catalog` issues and the wish list in `docs/catalog-sources.md` and opens
  one pull request.
- **GitHub:** public; issues #1-#7 roadmap (#10, the phone layout, is
  closed); Discussions on.

## Guiding principle (the maintainer, 2026-09-18)

Easy enough that someone new to all this understands it without being
confused, and configurable enough that an expert enjoys it. Sensible defaults
and plain words on the surface; everything adjustable in Settings.

## Decided 2026-09-18 (the maintainer's picks)

1. Layout: one page with a sticky jump bar (not tabs, not a sidebar).
2. Top of page: a row of to-do cards (updates, older copies, archive, won't
   boot); "Review" opens a checklist instead of filtering the list.
3. Checking: automatic on open and after scans, results remembered for a day,
   one Refresh button; a setting turns automatic checks off.
4. Statuses: plain words ("Update ready", "Old release", "Check by hand",
   "Won't boot from here") with a hover explanation and a key.
5. Filters: one Filter menu with checkmarks, active filters as removable chips
   plus "Clear all".
6. Rows: slim; file and size under the name, version as "old → new", one
   action.
7. Actions: a details panel for each image (links, switch, Update, Remove).
8. Update dialog: Replace pre-selected, sizes as the change ("about +6 MB"),
   "Don't ask again" (turned back on in Settings). This relaxes the "every
   delete asks" rule for updates only; update docs/design.md when built.
   *Built in v0.3.0 (each image carries its own choice, so updates never ask)
   and the hard rules in docs/design.md and SECURITY.md now say so.*
9. Settings: Light/Dark/System, high contrast, larger text, reduce motion.
10. Records: the user chooses per folder (in the folder, in isoshelf's folder,
    or a folder they pick), plus a remembered-folders list with last used,
    size and Forget.
11. Date column: "Updated …" for images isoshelf replaced, else "Added …".
12. Catalog: fill the gaps in DistroWatch's 12-month top 50, up to three a
    week through the routine, each checked on the project's own site.
13. isoshelf updates itself with one click ("Update now"): waits for
    downloads, swaps its own program, restarts, and the page reconnects.
14. Releases are signed; isoshelf only installs updates carrying that
    signature. One-time setup by the maintainer (a key made by a small tool in
    this repository, the private half stored as a GitHub secret).
15. More settings: empty the archive after 7/30/90 days, a download speed
    limit, hide architectures and kinds you don't use, a notification when
    downloads finish.
. "Images that were here" is split in two. **Archive**: only files still on
    the drive (size, Restore, Delete for good, Empty archive), with a to-do
    card and a jump-bar link. **History**: a folded log of images that left
    (deleted, replaced, vanished) with Download again. "Archive" stays the
    word everywhere.

## Where to pick up

`docs/TODO.md` is the running work list: what to do next, in order, with what
a cold session needs to know before starting each one. This file stays the
record of where things stand and what was decided.

## Next, in order

1. v0.3.0: the redesign (1, 2, 4-9, 11, 15, 17), including the phone layout
   (#10). *Done.* v0.3.1: the Settings panel (9). *Done.*
2. v0.3.x: automatic remembered checks (3). *Done in v0.3.2.* Next: records
   location and remembered folders (10; `internal/settings` holds the fields
   already);
   the rest of decision 15 (empty the archive after so many days, a download
   speed limit, hiding kinds and architectures) now that Settings has a home
   for them; one-click self-update with signed releases (13, 14).
3. v0.4.0: `isoshelf update` (#1), older versions with a hold (#2), Make
   bootable (#3), signatures on images (#5), two downloads at once (#4),
   portable test (#7).
4. Later: server mode in Docker (16).

## Latest change (2026-09-21)

- Decided 2026-09-21: **v0.4.0 includes redoing the page.** The maintainer's
  words were "clean up the UI - make it look like something I could show an
  investor", driven by the page not looking modern or polished rather than by
  any particular screen, and starting fresh rather than from a list of
  complaints. Not before the 0.3.x items, which stay the priority. Higher
  contrast, larger text, less movement and the phone layout all have to
  survive it.
- v0.3.4 splits the one `s.run` slot a scan and a download shared into
  `s.scanning` and `s.downloading`, so a scan or Refresh is no longer refused
  while a queue runs and the page shows both at once. Switching folders and
  emptying the archive still wait for downloads; scans wait only for scans.
- Underneath it was a second, worse bug: a scan loaded the folder's records
  at the start and saved them wholesale at the end, so a file a download
  placed meanwhile lost its record and could be listed as an image that had
  left. `state.SaveOnto` gives a scan the merge every other writer already
  used. It was reachable before the slot was split.
- Next: where each folder's records live and the remembered-folders list
  (decision 10), then isoshelf updating itself (13, 14).
- The v0.3.x branch is merged and released. [#13] went in as `9f609a6` and
  the release-workflow fix [#14] as `c49b13a`; `main` now carries everything
  through v0.3.3 and both branches are deleted. The release itself was
  started by hand from the Actions tab, because the sandbox refuses `v*` tag
  pushes with a 403, and it attached the five versioned files as intended.
- Issue #10 (the phone layout) is closed; it had shipped in v0.3.0.
- Next: v0.3.4, scanning while downloads run - splitting the single `s.run`
  slot the scan and the download share. `docs/TODO.md` has what it needs.
- v0.3.3 worked through the maintainer's own review of the page on a full
  drive (`isoshelf_v0.3.1_tasks.md`): the Filter menu now stays inside the
  window and its tick boxes line up (it was the one menu never wired to
  `placeMenu`); a download that would land on top of a file asks "Archive the
  old one" or "Replace it" instead of failing with advice; American spelling;
  "Restore" in place of "Put back"; tidier Archive and History cards with no
  redundant "Download again"; and two numbers people kept asking for - what
  the images use, and how much the queue still has to download.
- Released so far: v0.2.0 through v0.3.3, twelve of them. v0.3.1 and v0.3.2
  have their own changelog sections but were never tagged, so they went out
  inside the v0.3.3 release.
- Release files now carry the version; the copies inside the portable zip
  keep the plain name, since that is the one run from the drive and the one
  self-update will replace in place. Anything that downloads an update must
  match its asset by pattern, not by an exact name.
- Six specialized agents now live in `.claude/agents/`: `explorer` (find
  things, haiku), `worker` (small specified edits), `page` (the web page),
  `words` (UI text and docs), `bugs` (reproduce and diagnose), `core` (the Go
  that reads and reasons). Architecture, anything writing to a drive, and
  decisions stay with the main session. CLAUDE.md also says plainly when
  delegating costs more than it saves.
- Still owed from that review: scanning and checking while downloads run
  (needs the server's single run slot split in two - its own step, v0.3.4),
  and uploading a file to the drive from the browser (its own step too, since
  it writes to the drive).
- v0.3.2 shipped checking by itself: every scan is also a check unless
  Settings says otherwise, and `internal/lastcheck` keeps each project's
  answer in `<config>/last-check.json` for a day, so opening the page a
  second time asks nobody. One Refresh button replaced Scan and Check for
  updates; Settings gained "Check for updates by itself" and "Tell me when a
  new isoshelf is out" (which was a command-line flag only). Failures are
  never remembered, and nothing remembered is trusted for a download - a
  download still resolves everything again.
- Also: the three r/Ventoy cases the maintainer found are issues #11 (rebuild
  a drive from what isoshelf remembers - the config-folder mirror already
  holds the list) and #12 (move a drive's images to a bigger drive). Both are
  the same feature and the same 2026-09-21 decision.
- v0.3.1 shipped Settings: one searchable panel (theme, higher contrast,
  larger text, less movement, what happens to the files updates replace,
  where things are, Report a bug, Reset to defaults). Settings live in
  `<config>/ui.json`, which `internal/settings` now owns alone - the web
  server's second copy of the fields could erase what the command line
  wrote. The page's colors are written once with `light-dark()` and its
  sizes in `rem`, which is what makes a theme and larger text one line each.
  The panel is its own file, `internal/web/static/settings.js`.
- Then a clear-out: everything the dead-code checkers (x/tools `deadcode`,
  `staticcheck`) name as unreachable is gone, both come back clean, and `app.js`
  is no longer one 2,544-line file. The page is eight scripts, one per part
  of it, none over ~700 lines: reading one corner no longer means reading all
  of it. A line-by-line comparison showed nothing lost in the move, and the
  whole page was exercised in a browser afterwards.
- Docs checked against the code: CLAUDE.md and design.md describe the new
  script layout, CONTRIBUTING says how to add one, the "…" row menu the
  redesign removed is out of design.md, and the app-update notice is
  described as what it is (a link in the top bar and in Settings).
- That gap is closed: the app-update check now has its switch in Settings,
  and the command line reads the same answer.
- v0.3.0 shipped the new look: jump bar, to-do cards, plain statuses with
  explanations, one filter menu with chips, slim rows, a details panel,
  Archive and History apart, a per-image choice for old files, a checklist
  for "update all" and "older versions", a version field for images whose
  name doesn't say, and a phone layout.
- Decided 2026-09-21 (reviewing every feature against "simple defaults,
  everything in Settings"): backup = export the list, rebuild from it, and
  verify a copy (no file copying of our own); several drives kept alike =
  named Sets a drive can follow (no two-way sync); reuse = copy from folders
  you name, with other isoshelf instances later alongside Docker; drop
  version numbers in fixed filenames (breaks Proxmox VMs, fragile matching);
  "Make bootable" starts with renaming only; no notification when downloads
  finish; phone access and the QR code come with Docker; isoshelf will not
  empty a NAS recycle bin, only explain it.
- Next: v0.3.2, checking by itself (decision 3) - on open and after a scan,
  the answer remembered for a day and written down so a restart keeps it, one
  Refresh button, and the switch for it in Settings, which is now there to
  put it in.

[#13]: https://github.com/ZachCurry13/isoshelf/pull/13
[#14]: https://github.com/ZachCurry13/isoshelf/pull/14
