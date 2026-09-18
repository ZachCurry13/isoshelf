# Status

## Where things stand (2026-09-18)

- **Released:** v0.2.8. Scans a folder, works out each image, checks for
  updates online, downloads and verifies (resumable, queued one at a time,
  reorderable), replaces or archives old files, identifies unknown files, and
  keeps a catalog that updates itself from this repository. Web page and CLI;
  Windows and Linux builds from a `v*` tag.
- **Catalog:** 86 entries (60 downloadable, 9 update-check only, 17 link only),
  revision `2026091804`. All 69 online entries and all 60 images answered
  live on 2026-09-18.
- **GitHub:** public; issues #1-#7 roadmap, #10 phone layout; Discussions on.
- **Known problems:** during downloads, Remove, Archive, "What is this?" and
  "Clear older copies" wait; the row "…" menu is cut off inside the table;
  "Show them" on older copies leaves a filter on with no obvious way back;
  archived files only show in a folded section at the bottom.

## Decided 2026-09-18 (the maintainer's picks for the redesign)

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
12. Catalog: fill the gaps in DistroWatch's 12-month top 50 (about 25), in
    batches, each checked on the project's own site.

## Next, in order

1. v0.2.9: downloads stop blocking Remove, Archive and "What is this?" (a
   three-way merge of the folder's state file instead of refusing).
2. v0.3.0: the redesign above (1, 2, 4-9, 11), including the phone layout (#10).
3. v0.3.x: automatic remembered checks (3); records location and remembered
   folders (10; `internal/settings` has a start).
4. Catalog batches from item 12.
5. v0.4.0: `isoshelf update` (#1), older versions with a hold (#2), Make
   bootable (#3), signatures (#5), two downloads at once (#4), portable test (#7).

## Waiting on the maintainer

- Whether and how to split the files over ~300 lines (listed 2026-09-18).
