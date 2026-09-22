# Status

## Where things stand

Which version is the newest is deliberately not written here - it went stale
four times in one evening. The releases page on GitHub is the answer, and
`CHANGELOG.md` says what each one held.

- **What it does:** scans a folder, works out each image, checks for updates
  online, downloads and verifies (resumable, queued one at a time,
  reorderable), replaces or archives old files, and identifies unknown ones.
  Images can be added from the user's own computer as well as downloaded. A
  scan and a download run side by side, and every writer merges its own
  change onto the folder's records rather than saving over them. Keeps a
  catalog that updates itself from this repository. Web page and CLI;
  Windows and Linux builds.
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
2. v0.3.x: automatic remembered checks (3). *Done in v0.3.2.* Where a folder's
   records live (the first half of 10). *Done in v0.4.2.* Next: the
   remembered-folders list with last used, size and Forget (the rest of 10);
   the rest of decision 15 (empty the archive after so many days, a download
   speed limit, hiding kinds and architectures) now that Settings has a home
   for them; one-click self-update with signed releases (13, 14).
3. Server mode in Docker (16). *Done in v0.4.0.* What is left of it is a
   scheduler that checks by itself, and the official TrueNAS store app that
   is the 1.0 goal.
4. v0.5.0: the page redone, plus `isoshelf update` (#1), older versions with a
   hold (#2), Make bootable (#3), signatures on images (#5), two downloads at
   once (#4), portable test (#7).

## Latest change (2026-09-22)

- **isoshelf is running on the maintainer's NAS**, installed as a TrueNAS
  custom app from the published image. The two things that first install
  found are v0.4.1: the page scrolled sideways on a phone, and the refusal
  page told a container user to go back to a window that doesn't exist.
- The phone fix was measured, not guessed - at 320 pixels the document was
  358 wide. A grid item won't shrink below its own content unless told it
  may, and the catalog asked for 280-pixel columns on a 296-pixel page. A
  test now fails the build on any grid minimum written without `min()`.
- The container image is published and anonymously pullable (checked, both
  architectures, both tags), so items 2 and 3 of TODO's open questions are
  settled and the TrueNAS folder's checklist has two steps struck off.
- `deploy/truenas/` names the version it packages in two files; a test now
  fails the build when either falls behind the newest `CHANGELOG.md` section,
  because that is exactly the kind of thing nobody remembers at release time.
- v0.4.1 is released and the image is rebuilt and anonymously pullable, both
  architectures, `latest` and the version tag.
- v0.4.2 is the first half of decision 10: where a folder's records live, per
  folder, with the answer in Settings under *Where things are*. `state.Home`
  resolves it and `state.Move` moves what isoshelf knows when the answer
  changes - a folder that came back forgotten would be worse than not
  offering the choice. Two things fell out of building it: a records file
  kept away from its folder must be named from the folder's path, because the
  target id lives inside the file you are looking for; and only the records
  may move, since archiving is a rename and a rename across disks copies
  every byte.
- design.md's server-mode section said "later, not started". It shipped in
  v0.4.0; it now says what is done and what isn't.
- Next: the rest of decision 10 - the remembered-folders list with last used,
  size and Forget.

## Earlier (2026-09-21)

- Numbering settled: the container work is **v0.4.0**, the redesign moves to
  **v0.5.0**, and an official TrueNAS app is the **1.0** goal.
- Three things found by running the binary as TrueNAS's own app user (568)
  rather than assuming, all fixed: the docs said 1000 and would have caused
  the very permissions failure they warned about; a config folder isoshelf
  couldn't read made it refuse to start instead of falling back to the
  built-in catalog; and a folder named at startup that wasn't there produced
  no complaint at all, which in a container is a mistyped mount and nothing
  to go on.
- The secret in the link now makes itself on a server and survives a restart,
  so installing it asks nobody to invent a password for a box that is the
  only thing between a stranger and their images.
- `deploy/truenas/` has the start of a store entry, written against the real
  schema of an existing community app. Two files are deliberately absent -
  see that folder's README.
- v0.3.7 reverses v0.3.6's keep-both direction at the maintainer's request:
  the **new** download carries the version in its name and the file already on
  the drive is not touched. Their call, and the safer one - nothing that
  exists is renamed, so nothing pointing at a file by name can break.
- v0.4.0 is server mode and the container. `--listen ADDRESS` turns off the
  localhost-only rule deliberately; the token, cookie, custom header and
  same-origin check all stand, and the origin check now accepts `https` for a
  reverse proxy. `/healthz` is the one path outside the guard. `ISOSHELF_TOKEN`
  keeps a bookmark working across restarts.
- v0.3.7 and v0.4.0 were held on a local branch that evening at the
  maintainer's instruction, then split into named branches and released
  separately the next morning.
- Decided 2026-09-21: **error reports go to GitHub, not to a server.** The
  page opens a prefilled issue form and the user submits it. No backend, no
  telemetry, nothing to opt out of.
- v0.3.6: keeping both copies of a fixed-name image, reporting a problem,
  two menu bugs, and a plainer set of words.
- **Keep both was refused, and that was wrong.** `internal/update/keepboth.go`
  renames the old copy (its version when known, otherwise the day it arrived)
  and lets the new download take the unchanging name. That direction is the
  point: decision 2026-09-21 rejected version numbers in fixed filenames
  because it breaks Proxmox VMs - which it does, if the *new* file is
  renamed. Renaming the old one leaves whatever points at that name working
  and quietly newer. The renamed file's record is marked assigned so the next
  scan still knows what it is.
- **Reporting sends nothing.** `/api/report` says only which isoshelf, which
  system and what kind of folder; the page opens GitHub's bug form prefilled
  and the user submits it. No server, no telemetry, nothing to opt out of. A
  test proves the folder's path and its filenames never appear in it.
- v0.3.5 adds a file from the user's own computer: dragged onto the page or
  chosen, streamed straight to the folder through `internal/upload`. Image
  files only, inside the chosen folder, and a name clash asks the same two
  questions an update asks. It lands in `.isoshelf/incoming` and is renamed
  into place only when all of it is there.
- It uncovered an older bug, fixed with it: a file replaced by one of the
  same name was moved aside correctly, but the next scan forgot the note
  because that name was in the folder again, so it vanished from the archive
  while still using room. The archive now lists what is really waiting in
  `.isoshelf/removed`, and the archive/history split is about whether the
  file is still there rather than whether it can go back right now.
- Decided 2026-09-21: **nothing in the repository may say something that
  isn't true, checked before every push.** The README named a download by its
  versioned filename, which went stale every release. Names are now given by
  the part that doesn't change, the rule is in CLAUDE.md, and `internal/docs`
  fails the build if a versioned release filename reappears.
- Decided 2026-09-21: **v0.5.0 includes redoing the page** (v0.4.0 when this
  was decided; the container work took that number instead). The maintainer's
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
  started by hand from the Actions tab, and it attached the five versioned
  files as intended.
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
