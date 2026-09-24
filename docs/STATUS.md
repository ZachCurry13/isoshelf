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
  revision `2026092301`. Checked live every Monday by
  `.github/workflows/catalog-check.yml` and on every catalog pull request; a
  weekly Claude routine ("isoshelf weekly catalog", Mondays 18:00 UTC, 1 pm
  Chicago, just after the maintainer's weekly usage resets;
  https://claude.ai/code/routines/trig_01UzAxtUxF8jd4RwavQXAYdL) works through
  `catalog` issues and the wish list in `docs/catalog-sources.md` and opens
  one pull request.
- **GitHub:** public; open issues #1-#7, #11, #12, #56-#58 and #62 are the
  roadmap (#10, the phone layout, #54, the simpler shelf, #55, missing
  images, and #59, gentler with servers, are closed); Discussions on.

## Guiding principle (the maintainer, 2026-09-18)

Easy enough that someone new to all this understands it without being
confused, and configurable enough that an expert enjoys it. Sensible defaults
and plain words on the surface; everything adjustable in Settings.

## Decided 2026-09-18 (the maintainer's picks)

1. Layout: one page with a sticky jump bar (not tabs, not a sidebar).
2. Top of page: a row of to-do cards (updates, older copies, archive, won't
   boot); "Review" opens a checklist instead of filtering the list.
   *Replaced in v0.7.0 by one line of counts that filter the list, with
   Update all its one button (decided 2026-09-23).*
3. Checking: automatic on open and after scans, results remembered for a day,
   one Refresh button; a setting turns automatic checks off.
4. Statuses: plain words ("Update available", "End of life", "Download
   manually", "Won't boot from here") with a hover explanation and a key.
5. Filters: one Filter menu with checkmarks, active filters as removable chips
   plus "Clear all".
6. Rows: slim; file and size under the name, version as "old → new", one
   action.
7. Actions: a details panel for each image (links, switch, Update, Remove).
8. Update dialog: Replace pre-selected, sizes as the change ("about +6 MB"),
   "Don't ask again" (turned back on in Settings). This relaxes the "every
   delete asks" rule for updates only; update docs/design.md when built.
   *Built in v0.3.0 (each image carries its own choice, so updates never ask)
   and the hard rules in docs/design.md and SECURITY.md now say so. Since
   v0.7.0 it is one answer in Settings and a pin per file.*
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
16. "Images that were here" is split in two. **Archive**: only files still on
    the drive (size, Restore, Delete for good, Empty archive), with a to-do
    card and a jump-bar link. **History**: a folded log of images that left
    (deleted, replaced, vanished) with Download again. "Archive" stays the
    word everywhere.

## Where to pick up

`docs/TODO.md` is the running work list: what to do next, in order, with what
a cold session needs to know before starting each one. This file stays the
record of where things stand and what was decided. Finished work from both,
and the write-ups of releases before the latest, are in `docs/archive.md`.

## Next, in order

`docs/TODO.md` ("Right now") is the one list of what comes next. This section
used to keep a second one, and it went stale: it still called self-update,
the archive timer and the update scheduler "next" after all three shipped.

## Decided 2026-09-23 (the maintainer, clearing the issue list)

- **v0.5.0 is polish and correctness only**: the look, [#6] (Fedora stops
  pinning a release), emptying the archive on a timer, the tools list, and a
  confirmation before a folder's records move. Their words: "it should look
  like it could actually be a 1.0.0". Nothing half-built goes in.
- **[#5], OpenPGP signatures: build it, and accept the second dependency**
  (`github.com/ProtonMail/go-crypto`). The one-dependency rule has been worth
  defending - `golang.org/x/term` was turned down for password echo a few days
  ago - but a checksum proves nothing if the checksum file itself was
  replaced, and "only from the project's own HTTPS site" is a rule about where
  a file came from, not about whether it is genuine. A failed signature must
  block placement exactly like a mismatch does.
- **[#3], make bootable: rename and extract only.** A ChromeOS-family `.bin`
  renamed to `.img`, and `.img.xz`/`.zip`/`.gz` extracted - both small,
  standard library, original kept until the result is checked. **Not**
  `.bin`/`.cue` to `.iso`: raw sectors and multi-track cue sheets, where being
  subtly wrong gives a file that looks finished and won't boot. isoshelf says
  what it can't fix and names `bchunk` instead.
- **[#7] is the maintainer's to do** - it needs a real USB stick. A nine-step
  checklist is on the issue; steps 5, 6 and 9 are the ones most likely to
  fail.

## Decided 2026-09-23 (default credentials)

- The maintainer asked for documented default logins to be noted - "user
  demo: demo, user root: root", which is MX Linux's live-session default, so
  the case is real. **The rule is written down and the feature is not built**,
  deliberately: filling the field means reading each project's own
  documentation, and every such site tested from these sessions is refused by
  the egress proxy. An empty field would show nothing; a filled-from-memory
  one would put unverified passwords in front of people.
- The rule, in `docs/design.md`: the project's own documentation and nowhere
  else, the same as checksums. It rots, so the entry carries the address it
  was read from and the page shows it. Shape and reasoning in TODO item 19;
  the weekly catalog job is where it gets filled.

## Decided 2026-09-23 (the maintainer, using v0.5.1 on a phone)

- Moving records asks in a dialog (both files, what stays), says what
  happened in the setting's own note, and in the "records already there" case
  keeps today's rule - those win, the others stay - but says so, with dates.
- Filter menu: ticking still filters at once; a footer adds Clear all and
  Done. Not Apply/Cancel.
- When every update is one to fetch by hand, the card names that job ("1
  update to download yourself") and "Show it" shows exactly those.
- The name column was the one too wide.
- One-click rename stays with [#3], after v0.5.0 as planned; the note stops
  promising a button and says what to do by hand.
- The six unreleased versions went out one at a time, each on its own merge
  commit, v0.5.1 last.

## Decided 2026-09-23, evening (the maintainer)

- **Self-update** (decisions 13 and 14): downloads only when Update now is
  pressed; portable mode replaces all three programs; a single download takes
  the plain name on its first update. Built first, ahead of [#6] and item 18.
- **Queued behind it, in order:** upload speed; "Download again" and "Stop
  expecting it" for missing images; dismissing an update for 7/30/90 days or
  forever (any update, the timer holds, listed in Settings with Undo); a
  "where to find it" note for images fetched by hand; noting duplicate
  images and offering to delete a copy. Details in TODO.
- DistroWatch's download links are a lead for the weekly catalog job, never a
  source ([#50]). MX Linux 32-bit fixed in the catalog ([#51]).
- The six unreleased versions, and v0.5.2 and v0.5.3, were released one at a
  time on their own merge commits.

## Decided 2026-09-23, late (the maintainer, on a simplification plan)

- **Pin replaces the per-image Replace / Archive / Keep both menu**: one
  global answer plus a pin that keeps an exact file. Pinned files are left out
  of the older-versions review; Remove still works and says the file is
  pinned; "Keep both" images become pinned, the rest follow Settings, and the
  page says once how many changed.
- **The to-do cards become a compact bar** that filters, keeping only Update
  all as a button.
- **The architecture badge shows only when it isn't ordinary 64-bit PC.**
- **In a container, isoshelf asks once** whether to update images
  automatically; everywhere else it stays off by default.
- Order: v0.6.1 is upload speed and the good-neighbour pass; v0.7.0 is this;
  then the rest of the queue. Details in TODO.

## Decided 2026-09-24 (the maintainer, using v0.6.0)

- **Logo:** a disc on a shelf, picked from three; the stacked bars read as a
  menu button.
- **Server folder card:** one line (checked, space, Refresh), with the
  folder, its type and Choose folder in Settings. In v0.7.0 with the rest of
  the page top.
- **Linux install:** a one-line script, `install.sh`, in the README.
- **The exe stays the Windows download**; no installer.
- Sharing belongs to servers only; the Settings footer says where settings
  really are.

## Decided 2026-09-24, later (the maintainer, on pulling from the server)

- **Checking a file is only ever done when asked** - "A lot of times I just
  trust it" - with the page saying plainly which files haven't been checked.
  The same answer settles duplicates (queue item 6).
- **Pulling any image from the server**, catalog or not, with a record of
  where each file and its checksum came from: item 18 in `docs/TODO.md`,
  [#62], next after missing images. Asked to "make it look like" a file was
  checked against its source; declined, since a false record is worse than
  none and a real check is there for every catalog image.
- **Saying yes to updating by itself starts the first run at once**, as the
  switch always has, and the container's question says beforehand what that
  run will do.
- **[#62]'s layout:** the server's images in Add images (an "On your server"
  mark, and a folded "Also on your server" list), where each file came from
  in the details panel only, and copies with no published checksum allowed,
  marked as matching the server's copy.

## Documents brought into line (2026-09-24, no release)

A review of the documents against the code. README: the Why list leads with
the main features and puts the rest a line each; the per-release roadmap
paragraphs went (CHANGELOG.md holds them); *Getting it* now holds the
first-run paragraph and the Linux USB note. SECURITY.md: server mode and
sharing with another isoshelf are promises too. CONTRIBUTING.md stops listing
page scripts (it had 9 of 14), and `internal/docs` now checks it and
SECURITY.md as well. CLAUDE.md's layout names every package.

Second pass: the secret link was still called "the way back from a forgotten
password" in `docker-compose.yml` and the TrueNAS notes, though v0.4.7 ended
that; a server with no login yet is open to whoever arrives first, which
SECURITY.md and the README now say plainly. design.md stops listing
dependencies and image signatures as if built, and its app-updates section
stops saying "only a notice".

Then TODO.md and STATUS.md shed their finished work into the new
`docs/archive.md` (about 750 and 650 lines down to about 225 and 365). TODO.md
keeps its item numbers, since decisions refer to them.

## Latest change: v0.8.0 (2026-09-24)

- **Where each file came from** ([#62], part one): every file's records say
  how it arrived and, when its hash matched a published checksum, which one
  and when; the details panel shows it.
- **Check it** reads one file on request and compares it with the published
  checksum, going online if the answer isoshelf has is stale.
- **Next:** [#62] part two, copying any image from the server.

Earlier releases, v0.7.1 back to the start, are written up in
[archive.md](archive.md).

[#3]: https://github.com/ZachCurry13/isoshelf/issues/3
[#5]: https://github.com/ZachCurry13/isoshelf/issues/5
[#6]: https://github.com/ZachCurry13/isoshelf/issues/6
[#7]: https://github.com/ZachCurry13/isoshelf/issues/7
[#50]: https://github.com/ZachCurry13/isoshelf/pull/50
[#51]: https://github.com/ZachCurry13/isoshelf/pull/51
[#54]: https://github.com/ZachCurry13/isoshelf/issues/54
[#59]: https://github.com/ZachCurry13/isoshelf/issues/59
[#55]: https://github.com/ZachCurry13/isoshelf/issues/55
[#62]: https://github.com/ZachCurry13/isoshelf/issues/62
