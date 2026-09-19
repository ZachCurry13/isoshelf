# Status

## Where things stand (2026-09-18)

- **Released:** v0.2.9. Scans a folder, works out each image, checks for
  updates online, downloads and verifies (resumable, queued one at a time,
  reorderable), replaces or archives old files, identifies unknown files, and
  keeps a catalog that updates itself from this repository. While downloads
  run, removing, archiving and identifying still work (the state file is
  merged, never overwritten). Web page and CLI; Windows and Linux builds.
- **Catalog:** 86 entries (60 downloadable, 9 update-check only, 17 link only),
  revision `2026091804`. Checked live every Monday by
  `.github/workflows/catalog-check.yml` and on every catalog pull request; a
  weekly Claude routine ("isoshelf weekly catalog", Mondays 13:00 UTC,
  https://claude.ai/code/routines/trig_01UzAxtUxF8jd4RwavQXAYdL) works through
  `catalog` issues and the wish list in `docs/catalog-sources.md` and opens
  one pull request.
- **GitHub:** public; issues #1-#7 roadmap, #10 phone layout; Discussions on.

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
16. Server/Docker mode, when it comes: mostly hands off, updating images and
    the catalog on its own on a schedule.

## Next, in order

1. v0.3.0: the redesign (1, 2, 4-9, 11, 15), including the phone layout (#10).
2. v0.3.x: automatic remembered checks (3); records location and remembered
   folders (10; `internal/settings` has a start); one-click self-update with
   signed releases (13, 14).
3. v0.4.0: `isoshelf update` (#1), older versions with a hold (#2), Make
   bootable (#3), signatures on images (#5), two downloads at once (#4),
   portable test (#7).
4. Later: server mode in Docker (16).

## Latest change (2026-09-18)

- v0.2.9: downloads stop blocking Remove, Archive, "What is this?", Put back
  and Clear older copies (`state.Merge`, `saveStateLocked`); menu no longer
  cut off; a way back from "older copies"; archiving says where files went.
- Text files audited line by line; ten stale facts fixed (roadmap, profiles,
  portable contents, SECURITY versions, the wish list).
- Weekly catalog check (GitHub Actions) and weekly catalog routine set up.
