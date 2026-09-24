# Archive

Finished work, moved out of `TODO.md` and `STATUS.md` so those two stay about
what is left and where things stand. Nothing here is a plan; it is kept for
what was learned doing each piece and why it was done that way. Version
numbers and file names are as they were at the time.

## From TODO.md

### What v0.5.0 held

**What v0.5.0 held, decided 2026-09-23** (the maintainer: "I want to make
sure we clean up everything we can before posting 0.5.0. Like it should look
like it could actually be a 1.0.0"). **Polish and correctness only** - nothing
half-built, everything visible finished:

1. The look (this file's v0.5.0 section).
2. **[#6]** - the Fedora entries stop pinning a release number. This is the
   least 1.0 thing in the repository: when Fedora 45 ships, isoshelf keeps
   offering 44 and nothing fails, which is the kind of quiet wrongness that
   costs trust once somebody notices.
3. Emptying the archive on a timer (item 12).
4. The tools list (item 16).
5. ~~**Moving a folder's records asks first.**~~ *(done in v0.5.3.*
   `state.Plan` says what `state.Move` would do and `Move` is built on it;
   `/api/records/plan` serves it; `records.js` asks, and the setting's note
   says what happened. What was learned:
   - **`settings.Settings` holds maps, so a copy of it is not a copy.**
     `after := saved; after.SetRecordsFor(...)` changes `saved` too. Work out
     anything from the old answer before setting the new one - the existing
     test for moving records back caught it.
   - **`replaceChildren` and `append` write a null as the word "null"**; `el`
     skips it. `static_null_test.go` finds a null handed straight to one.
   - **The browser pane holds a dialog's `close` event until it draws.**
     With the pane in the background, closing a dialog from a script looked
     like a stuck page; a screenshot made it draw and every event fired. Test
     dialogs with real clicks, or take a screenshot before believing a hang.)*

20. **The badge said "x86" for a 32-bit image** *(done in v0.5.0.* The
   catalog was right - all ten of its `x86` entries say "(32-bit)" in their
   names - and the badge was drawing the raw value. It says the bit width
   now. Kept here because the lesson generalises: **the catalog's own words
   are identifiers, not labels.** Anything drawn from a catalog field should
   go through a map to what a person calls it.)*

### v0.8.5: `isoshelf update` on the command line ([#1])

*(released 2026-09-24)*: a check, then every update isoshelf can download,
through `update.Run` as the page does; `--keep`, `--move-aside` or
`--delete` with no default, `--only`, `--dry-run`; pins kept, dismissals
honoured unless named. The test runs it against a made-up project site, with
`--catalog` pointing at a catalog of one entry. On the way, the README's
promise that the command line only writes to `.isoshelf` had to become a
promise about `scan` and `check`.

### v0.8.4: duplicates, and a word about sharing (items 6 and 7)

*(released 2026-09-24, [#57])*. Possible duplicates by entry, version and
size, settled by hashes when every copy has one; a count on the line above
the list, a filter, and Make sure and Remove this copy on each row while it
is on. Make sure is `/api/hash`, an ordinary scan hashing those files, only
when pressed, as the maintainer decided. The sharing disclosure went beside
the switch and in the README (item 7), and on the way the container guide
turned out to name the switch by a name it never had.

### v0.8.3: where to find it (item 5)

*(released 2026-09-24, [#58])*. The catalog's `find` field, `{version}`
filled in with the newest version (or `VERSION` when the check hasn't found
one, which reads as a placeholder rather than a phrase spliced into a file
name), shown in the details panel above the links, where the maintainer
chose. The built-in catalog doesn't use it yet: see item 5 in `TODO.md`.

### v0.8.2: dismissing an update (item 4)

*(released 2026-09-24, [#56])*: for 7, 30 or 90 days or for good, from the
details panel; the row greyed and sorted with the up-to-date ones, out of
every count and of Update all and updating by itself; Settings lists every
dismissal with Undo and Undo all. What was learned:
- **`shortDate` only knows the past.** It called a date a month away
  "today", so a dismissal said "until today". `untilDate` is for days still
  to come.
- The dismissal lives on the image's track in the folder's records, beside
  its star, so it belongs to that folder: a laptop and a server can each
  dismiss what they like.

### v0.8.1: copying anything from the server (item 18, part two)

*(released 2026-09-24, [#62])*. `/api/share/list` on the sharing isoshelf,
`peer.Client.List`, `/api/peer/files` and `/api/peer/copy` on the laptop, a
copy job in the download queue (`runCopy`), and in Add images the "On your
server" mark and the folded "Also on your server" list (`fromserver.js`).
What was learned:
- **`s.catalog()` takes `s.mu`**, so calling it with the lock held hangs the
  request for good; the copy handler did, and the test sat there until it
  was stopped. Under the lock, read `s.cat`.
- **A server that found a file on its first scan knows nothing of when it
  got it** (the first scan sets no `FirstSeen`), so `Before` can be empty,
  and that is the honest answer; the page then says only that it came from
  the server.
- The newest release of a catalog image already came from the server
  through `Nearer`, checked against the project; a copy is for everything
  else, and the next check proves it if it happens to be the release.

The item as it was written, for the reasoning:

18. **Ask the server by entry, not only by hash** (the maintainer,
   2026-09-23, two questions that turn out to be one answer: "is it possible
   to have some sort of marker in the catalog that says there is a local copy
   on the server?" and "if there are files that have to be manually
   downloaded, but I have it on the server already, is it possible to just
   download from the server instead of doing another manual pull? Since I
   obviously did that already").

   **Why neither works today.** The whole sharing protocol is keyed on the
   checksum: `share/have?name=X&sha256=Y`, and `sharedFile` refuses without a
   hash. That is what makes it safe - the peer is reached, never trusted, and
   the bytes are checked against the project's own published checksum. A
   manual entry has no published checksum, so isoshelf cannot even form the
   question. Not an oversight; a consequence.

   **The one new endpoint both need.** Ask by entry id: "what do you have for
   `ubuntu-desktop`?" The answer is the filename, the server's own SHA256, the
   size, and the version it believes the file to be. One call with no entry
   given returns the lot, which is what the Add-images marker needs - one
   round trip for the whole list, not one per image.

   **The rule that has to hold.** Bytes copied this way are checked against
   the hash the server gave, which proves the copy is identical to what is on
   the server and proves nothing about provenance. So the file arrives
   **Unverified**, and the existing rule applies unchanged: an unverified
   download never replaces anything. Adding a manual image the folder doesn't
   have is fine; overwriting one is not. Say in the UI whose word it rests on
   - the person who put it there - rather than implying a check happened.
   - The server can also hand back the entry and version it has recorded, so
     the copy arrives identified rather than landing as "Unrecognized". That
     is the same kind of assertion as somebody naming a file by hand, and
     should be recorded as one.
   - Sharing is off unless turned on and the asker must be signed in. Both
     already true; neither changes.

   **Asked for again, and wider, 2026-09-24** (the maintainer: "pull any
   image locally from the server, even if it isn't in the normal catalog.
   When pulling any image, do we keep the verified checksums locally? ...
   making sure I pull the image from the correct source at least once"). It
   is [#62], and **next after missing images** (decided that day), since
   Download again can then try the server first.
   - **Any image**, not only catalog ones: the laptop lists what the server
     has that it doesn't, and copies it over.
   - **A record of where each file came from**, which isoshelf doesn't keep
     today. `FileRecord` has the file's own hash and, for a download, the
     address and date - and for a copy from the server that address is the
     server's. Keep instead: where the checksum came from and when it was
     checked against it, and where the bytes came from (the project's site,
     the server, added by hand). The server hands its record over with the
     copy, so the laptop can say "no published checksum; copied from your
     NAS, which has had it since March (added by hand)".
   - **Checking a file by hand against the published checksum**, for a
     catalog image that arrived any other way (a torrent, a USB stick). A
     match proves it is byte for byte the release, which is worth more than
     where it came from, and it is true. Only when asked, as for duplicates:
     until then the page says the file hasn't been checked, and offers to.
   - **Never a record of a check that didn't happen.** The maintainer asked
     for it to "at least make it look like I did"; that was declined, and
     said so. A false provenance record is worse than none - it is exactly
     what looks bad if anyone ever looks - and a real check is available for
     every catalog image anyway.

   **Decided 2026-09-24, and what is left.** The maintainer picked: the
   server's images show **in Add images** - a catalog image the server has
   gets "On your server" and its Add button copies from there, and files the
   catalog doesn't know go in a folded "Also on your server" list at the
   bottom; provenance shows **in the details panel only**; and a copy with
   no published checksum is allowed, **marked** as matching the server's
   copy, never replacing anything. Part one (v0.8.0) built the record,
   Where it came from and Check it. Part two is the rest: a list endpoint on
   the sharing isoshelf (every hashed file, with its entry, version and
   `Origin`), `peer.Client.List`, the marker and the folded list, and Copy
   for a file that isn't the newest release (the newest already comes from
   the server through the existing `Nearer` path, checked against the
   project), verified against the server's hash and recorded with `Before`.

   **Not in v0.5.0**, which is polish and correctness only. This changes what
   "verified" means at the edges and deserves a release where the
   unverified-copy rules get real tests, rather than riding along in one whose
   whole point is that nothing in it is half-built. First thing after, ahead
   of [#1] and [#4]: it is what makes running the server worth it.

### v0.8.0: where each file came from (item 18, part one)

*(released 2026-09-24, [#62])*. `state.Origin` on every file record, filled
in by downloads, uploads and any check that finds a file's hash is the
published one; Where it came from in the details panel; Check it. What was
learned:
- **The check could already prove a file**, and never said so. A fixed-name
  image is hashed every scan and compared with the published checksum, and
  a newest-release file with a hash is compared too - but the answer only
  ever became "up to date". `check.Item.Matched` keeps the checksum's
  address, and `inventory.Run` records it.
- **Remembered answers keep the checksum's address**, because `lastcheck`
  stores the whole `resolve.Artifact`; a Check it that reuses today's answer
  still says where the checksum came from.
- Check it rides on an ordinary run (`HashPaths`, `askToProve`) rather than
  a slot of its own, so the page shows its progress the way it shows any
  scan, and it can't run beside a scan reading the same disk.

### v0.7.1: missing images

**~~3. Missing images get two actions~~** *(released in v0.7.1, [#55])*: the
row's button gets the image back, and "Stop expecting it" in the details
panel is for one removed on purpose. As built:
- The button is **Restore** when the image's file is still in the archive,
  **Download again** when isoshelf can download it, else **Download page**
  (`missingAction` in `missing.js`). Restore first because it is instant and
  gives back the very file; nobody wants 5 GB downloaded to replace a file
  that is sitting in `.isoshelf/removed`. The rows are drawn before the
  archive is read, so `noteArchive` draws them again when it changes.
- Filtering to missing images offers **Download all…**, the Update all
  checklist again, leaving out any queued or waiting in the archive.
- **Stop expecting it** sets `Track.NotExpected` and unstars the image;
  `usualSet` leaves it out, and `RecordScan` clears it when a scan finds the
  image again. The server drops the row from the report at once
  (`dropMissingLocked`) - rebuilding the report with `check.Offline` would
  have thrown away the last check for everything else - and unstarring a
  missing image now does the same, where before it stayed until the next
  scan.
- In the details panel the button shows only when it does something the
  Links don't: a manual image already has its Download page there, and the
  first draft listed it twice.

### v0.7.0: the simpler shelf

**~~v0.7.0: the simpler shelf~~** *(released 2026-09-24, [#54])* (the
maintainer, 2026-09-23, after a review of a simplification plan written with
Gemini - whose sections on update prompts and background downloads described
things isoshelf already did). As decided:

- **Pin replaces the per-image Replace / Archive / Keep both menu.** One
  answer in Settings, plus a Pin per file: *keep this exact file*. An update
  downloads beside it; the new copy is not pinned. Pinned files are left out
  of the older versions, and automatic updates never touch them. Remove
  still works and says "This file is pinned." A pin belongs to the file, so
  it goes when the file changes. Migration: "Keep both" becomes pinned; any
  other answer of an image's own follows Settings; the page says once how
  many changed (`MigrateChoices`).
- **The to-do cards become one line** of counts that filter the list, with
  **Update all** the one button (`summary.js`).
- **The architecture badge only when it's unusual.**
- **In a container, isoshelf asks once** whether to update by itself (every
  day, every week, or no), off until answered.
- **On a server, the folder card is one line**; the folder, its type and
  Choose folder are in Settings under This folder.

What was learned:
- **`jumpTo` opened the Filter menu.** It opened the first `<details>` in
  the part of the page it jumped to, from when the archive folded away; the
  first one in the list is the Filter menu, so every "Show them" had opened
  it. It only scrolls now, and parts of the page stop below the sticky bar
  (`scroll-margin-top`) rather than under it.
- **Saying yes to updating by itself starts a run at once** (it always has:
  `AutoUpdateLast` is cleared so somebody sees it work). Found by answering
  the container's new question on a test folder and watching it set off to
  update all 31 images. The question now says what that run will do - how
  many updates, about how much to download, and what happens to each old
  file - and the maintainer chose to keep it starting at once.
- **The download sizes above the list were the files already there**, not
  the downloads: close on a real drive, 248 bytes on a folder of stand-ins.
  `downloadSize` takes the catalog's size, and the line is redrawn once the
  catalog has arrived.
- `actions.js` became `actions.js`, `catalog.js` and `identify.js`; the
  cards left `images.js` for `summary.js`. The badge test reads every script
  rather than naming two, so a split can't slip past it.

### v0.6.0 and v0.6.1

**~~v0.6.0, isoshelf updates itself~~** *(released 2026-09-23, signed; the
key is set up)* (item 10; the maintainer asked for it that day and put it
first). Decided then: it downloads only when
**Update now** is pressed, then checks the signature, waits for image
downloads, swaps and restarts on the same port; in portable mode it replaces
**all three** programs; a single download named for its version takes the
**plain name** on its first update. Not in a container (the image is updated
instead).

**Releases after v0.6.0** (decided 2026-09-23): **v0.6.1** is items 1 and 2
below (upload speed, already built on the `missing-and-more` branch, and the
good-neighbour pass). **v0.7.0** is the simpler shelf, next paragraph. Then
items 3 onwards.

The maintainer's queue after v0.6.0 began with these two:

1. ~~**Upload speed.**~~ *(done in v0.6.1: `uploadRate` in `upload.js`,
   smoothed the way the dock's is.)*
2. ~~**Be a good neighbour to the projects' servers**~~ *(done in v0.6.1,
   [#59]: `internal/remote/polite.go`. `Refused` reads `Retry-After` on a 429
   or 503 from any host; over `MaxPoliteWait` (a minute) it is a `BusyError`,
   which neither checks nor downloads retry. `NextWait` spreads the doubling
   wait between half and one and a half times itself, and each server picks
   an `autoOffset` of up to an hour for the automatic-update schedule.)*
   Also in v0.6.1, from the maintainer's own use: sharing shown and honoured
   only on a server (`sharing()` checks `AnyHost`), the Settings footer
   worded for portable and server, a new logo (a disc on a shelf, picked
   from three), and `install.sh` for Linux with a workflow that runs it on
   x86-64 and ARM Ubuntu weekly.

### v0.3.4: scanning while downloads run

A scan and a download had one `s.run` slot between them; they have one each
now (`s.scanning`, `s.downloading`), so a scan or Refresh is no longer
refused for as long as a queue takes. What was learned doing it:

- **There was a second bug underneath, and it was the worse one.** A scan
  loads the folder's records when it starts and saved them again wholesale at
  the end, so a download that placed a file in between lost that file's
  record and could be listed as an image that had left. `state.SaveOnto`
  (merge.go) now does for a scan what `saveMerged` already did for everyone
  else. This was reachable before the slot was split, whenever a scan
  followed a download closely enough, so it is a fix, not fallout.
- **Both directions are allowed**, not just the one the bug report named: a
  download can also start while a scan runs. Blocking that would have taken
  an extra guard, and the merge makes it safe either way.
- **A download reports its own progress now** (`downloads.current` carries
  `stage`, `done`, `total`), because `state.run` is the scan's card alone.
  `drawnKey` in `app.js` has to strip those three fields, or the whole page
  redraws twice a second and open menus close - the very thing that key is
  for.
- **Switching folders and emptying the archive still wait** for downloads
  (`busyLocked`); scans wait only for another scan (`scanBusyLocked`).
- `waitIdle` in the tests now waits for both slots; `state.run` going nil no
  longer means the queue is done.

### Answered 2026-09-21, late

1. **Numbering:** the container work is **v0.4.0**, the redesign moves to
   **v0.5.0**. Running on a NAS is a new ability, and the project's own rule
   gives those the middle number.
2. ~~**The unbuilt image:**~~ *settled:* the first `container image` run
   built it for both architectures and pushed it, and the Dockerfile needed
   no fixing.
3. ~~**ghcr.io starts private**~~ *settled:* the maintainer made the package
   public, and an anonymous pull of both architectures and both tags was
   checked rather than assumed.
4. **An official TrueNAS app is a 1.0 goal.** See below.

### "Then, in order": the items that shipped

4. ~~**Upload from the browser**~~ *(done in v0.3.5. `internal/upload` does
   the placing, `internal/web/upload.go` the endpoint - the request body is
   the file itself rather than a form, so it streams to the disk - and
   `static/upload.js` the page. What was learned: the file goes to
   `.isoshelf/incoming` and is renamed into place only once all of it has
   arrived, so a dropped connection costs nothing; and the page's content
   policy forbids inline styles, so a progress bar's width is set with
   `.style.width`, never a `style` attribute. It also turned up a bug older
   than itself - a file replaced by one of the same name disappeared from the
   archive at the next scan while still using room - fixed in the same
   release.)*
5. ~~**Run isoshelf on a NAS**~~ *(done in v0.4.0: `Dockerfile`,
   `docker-compose.yml`, `docs/docker.md`, and `--listen` with the guard
   changes behind it. What was learned: `os.UserConfigDir` honours
   `XDG_CONFIG_HOME`, so the container needed no code change to keep its
   files on a mount; and the same-origin check hardcoded `http://`, which a
   reverse proxy in front of a NAS would have broken.)*
6. ~~**Where each folder's records live**~~ *(done in v0.4.2. `state.Home`
   resolves it, `state.Move` moves them when the answer changes, and the
   answer is per folder, in the settings file under `folder_records`. What
   was learned: a records file kept away from its folder has to be named from
   the folder's path, because the target id lives inside the file you are
   trying to find; and only the records may move - archiving is a rename, and
   a rename across disks is a copy of every byte.)*
7. ~~**The remembered-folders list**~~ *(done in v0.4.3, which finishes
   decision 10. The mirrors in `<config>/targets/` gained `files` and `bytes`
   - written from the state at save time, absent in older mirrors and then
   simply not shown - and `state.Forget` removes one. The list lives in the
   folder chooser, where you are already deciding which folder to open. What
   was learned: a narrow column of full paths is useless, because the start
   is what every folder on one machine has in common; the end is what tells
   them apart.)*
8. ~~**Update the images by itself**~~ *(done in v0.4.6.
   `internal/web/autoupdate.go`: a schedule, the same download queue the
   Update button uses, and `removalFor`, which has to agree exactly with
   `choiceFor` in `details.js` - what happens unattended must be what the
   page has been showing. What was learned: the settings file had one writer
   and now has two, so every change to it has to take a turn, or a switch
   somebody flicks can be quietly undone by the scheduler noting the time.)*
9. ~~**Copy from another isoshelf on the network**~~ *(done in v0.4.9, the
   maintainer's feature request. `internal/peer` asks, `internal/web/share.go`
   offers, `update.Options.Nearer` puts the answer at the front of the
   download's list of places. What was learned: the peer is reached, never
   trusted - the checksum still comes from the project's own site, and
   `fetch` had to learn to fall through to the next place on a mismatch, or
   one bad copy would cost the whole download. Files are asked for by hash
   rather than name, so sharing makes scans hash everything. Still to decide:
   finding the other isoshelf by itself, which means mDNS, which means a
   second dependency or a lot of protocol code.)*
10. ~~**isoshelf updates itself**~~ *(built for v0.6.0; the shape is in
   docs/design.md under Releases.* **It can't ship until the maintainer has
   set up the signing key once** - `go run ./internal/appupdate/keygen
   -private release-key.txt -secret RELEASE_SIGNING_KEY`, which writes
   `release.pub` and stores the secret through `gh`; then commit
   `release.pub`, keep a copy of the file in a password manager, and delete
   it - because the release workflow now refuses to publish an unsigned
   release.
   What was learned:
   - **Windows can rename a running program but not delete it.** Undoing an
     update from inside the new program has to move it aside, not remove it;
     the first version removed it, and the end-to-end test failed with
     "Access is denied" when that was put back on purpose.
   - **isoshelf picks any free port by default**, so a restarted program has
     to be told the old one's port, or the page loses it. That, the staging
     folder and the old version travel in `ISOSHELF_HANDOVER`; the link's
     token in `ISOSHELF_TOKEN`, which isoshelf already read.
   - **The page's message line is redrawn on every refresh**, so a message
     shown from code that isn't an event handler vanishes in half a second.
     Anything that has to stay goes somewhere the redraw leaves alone - the
     top bar, here.
   - Not yet tried for real: downloading a signed release from GitHub and
     the page reloading after a real restart. The first chance is v0.6.0 to
     v0.6.1; watch that one.)*

12. ~~**Empty the archive after so many days**~~ *(done in v0.4.16.
   `internal/web/archivetimer.go`; it rides the auto-update tick rather than
   keeping a clock of its own. Every care listed below was built: off unless
   a number is chosen, each file judged by its own `GoneAt`, nothing touched
   that isoshelf has no archive record for, and Settings says what the next
   sweep would take before it takes it.)* (decision 15, and the
   maintainer picked this one next, 2026-09-22). 7 / 30 / 90 days or never,
   in Settings. Care needed: the archive is the undo for every removal and
   every replaced file, so emptying it on a timer is the one thing here that
   throws away something somebody might still want. It has to say what it
   will do before it does it, count from when each file was archived rather
   than sweeping the lot, and never touch a file archived since the last
   scan. The rest of decision 15 - a download speed limit, hiding kinds and
   architectures you don't use - can follow.

From item 13 (the maintainer's own use, 2026-09-22), the parts that shipped:

   - ~~**Say when a file arrived on the drive.**~~ *(done in v0.4.14: an
     Added column in the list, sortable, and still under the name on a
     phone. The date was already drawn under each name - what was missing
     was being able to sort by it.)*
   - ~~**The filter menu needs Apply and Clear all.**~~ *(done in v0.5.3:
     the maintainer chose Clear all and Done, with filters still applying as
     they are ticked - not Apply/Cancel.)*

Item 15, "v0.4 proper", listed [#1], [#2], [#3], [#4], [#5] and [#7] plus the
redesign; the redesign shipped as v0.5.0 and the issues moved to *After
those* in TODO.md.

16. ~~**The tools that go with the images**~~ *(done in v0.5.2. Ventoy,
   Rufus, balenaEtcher and Raspberry Pi Imager, in a folded section above the
   footer and under `### Tools that go with these` in README.md.
   `internal/docs/tools_test.go` checks the two name the same tools at the
   same addresses, and that neither has grown a version or a download.)*
   (the maintainer, 2026-09-22:
   "whatever happened to the author's suggested tools - Ventoy, balenaEtcher,
   Rufus. I really like those tools and people who have isoshelf would too").
   It was talked about and never written down anywhere, which is why it went
   missing; it is written down now. A short, hand-kept list of the tools
   somebody with a shelf of images actually needs - one that writes a drive,
   one that boots many images from one drive, one that checks a disc - each a
   line saying what it is for and a link to its own site.
   - `docs/design.md` already has the rule this has to follow: plain text and
     a link, never anybody's logo, and "independent project, not affiliated"
     next to it. No downloads, no versions, no update checks: the moment
     isoshelf tracks a tool's version it owns that tool's release notes
     forever, and this is meant to be four sentences that never go stale.
   - **Decided 2026-09-22** (the maintainer left it to me): both the README
     and the page. The README is where somebody deciding whether to use
     isoshelf reads; the page is where the people who already have it are,
     and they are the ones the maintainer meant. On the page it is a folded
     section low down, beside Archive and History, so it costs nothing until
     it is opened. Two copies would normally break the "don't write down
     anything that has to be maintained" rule - so a test in `internal/docs`
     checks the two name the same tools, and no version or download is
     tracked for any of them.
17. ~~**Say where a download is coming from**~~ *(done in v0.4.14.*
   `sourceName` in `internal/web/peer.go` turns the URL a download is using
   into the peer's own name or "the internet"; the dock shows it while bytes
   move and on the finished list. `fetch.Progress` already carried the URL,
   so nothing new is found out - it was only ever thrown away.)*
   (the maintainer, 2026-09-22:
   "when it is downloading, it would be nice to know if it was from the
   internet or from the local server"). Copying from another isoshelf
   (v0.4.9) is invisible while it happens: the dock says "Downloading 40%"
   whether the bytes are crossing the room or the Atlantic, and the whole
   point of the feature is that one of those is much faster. `fetch.Progress`
   already carries `URL` and `fetch.Result` records which URL won, so this is
   showing what is already known, not finding anything out.
   - Name the isoshelf, not the URL: "from nas.local", "from the internet".
     The peer's own name is in `internal/peer` (`FolderName`).
   - It also tells somebody their peer setting is doing nothing, which is the
     only way to find that out today short of watching a router.

### v0.5.0: the page, redone

The maintainer's brief, given 2026-09-21: **"clean up the UI - make it look
like something I could show an investor."** It was v0.4.0 when that was
decided; the container work took that number, because running on a NAS is a
new ability and the middle number is what rises for those. What was said when asked what drives it: the
page doesn't look modern or polished, and it is a fresh start rather than a
list of complaints about particular screens.

Read that as a design job, not a tidy-up. The bones were decided deliberately
(decisions 1-9 and 15-17 in STATUS.md: one page, a sticky jump bar, to-do
cards, plain status words, one filter menu with chips, slim rows, a details
panel) and v0.3.0 delivered them; what this asks for is the surface those
bones are wearing - type, colour, spacing, rhythm, polish - and a willingness
to overturn a decision where it earns it, by argument rather than quietly.

Three things it must not cost, because each has a switch in Settings and
someone relying on it: higher contrast, larger text, and less movement. Nor
the phone layout, nor the rule that text from the drive is inserted with
textContent and never as HTML.

Whatever is proposed, the maintainer wants to be asked about anything where
more than one answer is good, rather than shown a finished redesign.

**Asked and answered, 2026-09-22:**
- *Words:* use the ordinary ones. A Settings switch for "simple mode" was
  offered and turned down for now - two versions of every string across ten
  files is a cost that never stops - so the page uses the normal word and
  keeps its explanation in the hover and the second line. **Shipped as
  v0.4.13**, which is the wording half of this done.
- *Audience:* confident, not hand-holding. Same reasoning.
- *Look:* "whichever option feels like TrueNAS does when using" - so
  restrained palette, technical density: dark-first, dense, one blue accent,
  real tables.
- *The list:* keep the sortable table on desktop, cards on the phone.

What is left of v0.5.0 is therefore the visual work: type scale, colour
discipline, spacing and density. Show the maintainer what it looks like
before committing to it.

`docs/design-audit.md` is a critique of the page written before any of this
was built, with seven such questions already worked out and a staged plan.
Start there; it is a proposal, not a decision.

It also turned up two live bugs, ~~both fixed in v0.3.6~~: the Filter menu
ran off the left edge at phone width, and Escape didn't close an open menu.

### Answered questions

- ~~*Keep both* in the name-clash dialog~~ *(answered in v0.3.6, and the old
  answer was wrong: it can work. The new download carries its version in its
  name and the file already on the drive is not touched - the maintainer
  chose that direction in v0.3.7, over renaming the old file, because then
  nothing that exists is disturbed. See `internal/update/keepboth.go`.)*

## From STATUS.md: releases v0.8.4 and older

### v0.8.4 (2026-09-24)

- **Duplicate copies** ([#57]): counted above the list, filterable, with
  Make sure (reads the copies, only when pressed) and Remove this copy.
- **A word about sharing** beside the switch and in the README.

### v0.8.3 (2026-09-24)

- **Where to find it** ([#58]): a catalog entry can say where the file is on
  the download page, shown in the details panel above the links. The
  built-in catalog starts using it once older copies have had time to
  update, since they refuse an unknown field.

### v0.8.2 (2026-09-24)

- **Dismiss an update** ([#56]) for 7, 30 or 90 days, or for good: greyed,
  sorted with the up-to-date ones, out of every count and of Update all and
  updating by itself; Settings lists them with Undo.

### v0.8.1 (2026-09-24)

- **Copy any image from your server** ([#62], part two): Add images marks
  what the server has and copies it; files the catalog doesn't know are in a
  folded list. A copy is checked against the server's copy, recorded as a
  copy with the server's account of it, and never called checked for that.

### v0.8.0 (2026-09-24)

- **Where each file came from** ([#62], part one): every file's records say
  how it arrived and, when its hash matched a published checksum, which one
  and when; the details panel shows it.
- **Check it** reads one file on request and compares it with the published
  checksum, going online if the answer isoshelf has is stale.

### v0.7.1 (2026-09-24)

- **Missing images come back in one click** ([#55]): Restore when the file is
  still in the archive, Download again when isoshelf can fetch it, else its
  download page; Download all when the list shows the missing ones.
- **Stop expecting it** takes one off the missing list, and its star, until
  it is back in the folder. Unstarring a missing image now takes it off at
  once too.

### v0.7.0 (2026-09-24)

- **Pin replaces each image's own choice for old files** ([#54]): one answer
  in Settings, and a pin keeps one exact file whatever comes. Old answers
  become pins (keep both) or follow Settings, once, and the page says so.
- **One line says what wants doing**, each count a filter, with Update all
  its one button; the 64-bit badge only where it's unusual; on a server the
  folder card is one line and the folder lives in Settings; a container asks
  once whether to update by itself, saying what yes starts.
- Fixed on the way: clicking a count opened the Filter menu, the list it
  jumped to hid under the tabs, and download sizes were the files already
  there.

### v0.6.1 (2026-09-24)

- **Gentler with the projects' servers** ([#59]): `Retry-After` honoured from
  every host, a long one stops rather than retries, retries spread at random,
  and a per-server offset on the automatic schedule.
- **Adding a file shows speed and time left**; **a one-line Linux install**
  with a weekly workflow that runs it on x86-64 and ARM; **a new logo**.
- **Sharing only on a server** - it could never work on a desktop, and left
  on it made every scan hash everything - and the Settings footer worded for
  portable and server.
- v0.6.0 is the first signed release; v0.6.1 is the first update isoshelf
  installs by itself, so it's the one that proves Update now for real.

### v0.6.0 (2026-09-23)

- **isoshelf updates itself.** Update now in the top bar downloads the new
  version, checks the project's Ed25519 signature on `SHA256SUMS`, waits for
  image downloads, swaps and restarts on the same port; the page reconnects.
  If the new program can't come up it puts the old one back.
- Tested end to end on Windows by building and running the real program
  (`cmd/isoshelf/handover_test.go`); a real signed download from GitHub is
  the one thing not yet tried, and can't be until v0.6.1 exists.
- **Blocked on the maintainer:** the signing key (TODO item 10). The release
  workflow refuses to publish without it.
- Split on the way: `commands.go` (`ui.go`, `handover.go`), the web config
  (`config.go`).
- **Next:** the queue above, starting with upload speed.

### v0.5.3 (2026-09-23)

- **TODO item 5 is done: moving a folder's records asks first.**
  `state.Plan` says what `state.Move` would do, `/api/records/plan` serves
  it, and the page asks before anything moves and says after what happened.
- **The maintainer's report on v0.5.1**, all fixed: a lone filter chip
  followed by "null" (`replaceChildren` writes a null as text; a test now
  catches that), "Show them" showing the wrong images, a filter menu with no
  way out, a name column that took most of the table, and a won't-boot note
  that promised a button that doesn't exist.
- A flaky Windows test was a real race: the page state read the remembered
  folders before checking whether a scan was running. Fixed; 0 failures in
  600 runs, against 1 in 300 before.
- Four oversized files split with no change in behaviour: `settings.js`
  (into `settinglist.js`, `access.js`, `records.js`), `server.go` (`guard.go`),
  `records.go` (`forget.go`), `pagestate.go` (`pagedisk.go`).
- **Next:** [#6], then item 18 (asking the server by entry).

### v0.5.2 (2026-09-23)

- **The tools that go with the images** (TODO 16): Ventoy for booting many
  images from one stick, Rufus, balenaEtcher and Raspberry Pi Imager for
  writing one. A folded section above the footer, and the same four in the
  README under *Tools that go with these*.
- Plain text and a link each. No logos, no versions, no downloads: tracking a
  version would mean owning that tool's release notes forever.
- Two copies is what this repository normally refuses, so
  `internal/docs/tools_test.go` compares them by name and address and fails on
  a version or a download in either. It caught a real mismatch on its first
  run, and was checked by breaking a link on purpose.
- Two of the four addresses could not be reached from the sandbox (every one
  of the hosts is refused by the proxy); `ventoy.net` and
  `raspberrypi.com/software` are corroborated by text already in the repo.
- **Next:** [#6], the Fedora entries that pin a release number - it needs the
  network, so Actions or the policy change.

### v0.5.1 (2026-09-23)

- **"It would be awesome if I didn't have to log in every time."** Sessions
  already last 30 days; what was broken is that the key they are signed with
  was not surviving a restart, and nothing said so.
- `auth.LoadKey` now returns whether the key reached disk. It was being
  thrown away in three places at once: the write error inside `LoadKey`, the
  caller's `s.sessions, _ =`, and `writeSecret`'s own return. The only hint
  was a line on stderr, which nobody reads on a NAS.
- **The shape that matters is not an unwritable folder** - that stops the
  password saving too, so the symptom would be being asked to *set* one. It
  is a `session-key` left behind by a run under a different user, which is
  what happens when an app's user changes between versions. The account still
  saves; only the sessions die. Reproduced exactly that way, as a non-root
  user, before the fix and after.
- The page says it now, names the file, and says what to put right. Tests
  cover both halves, and the auth one skips under root and was run as an
  unprivileged user to prove it passes.
- Next: default credentials in the catalog (the maintainer, 2026-09-23), the
  tools list (TODO 16), and [#6].

### v0.5.0 (2026-09-23)

- **v0.5.0: the page redone.** The maintainer approved the direction from
  screenshots of the real page with a candidate stylesheet layered over it -
  which is the way to ask this question, because it costs nothing to say no.
- The brief was "restrained palette, technical density - not warm, not airy",
  and, said plainly afterwards, **not a copy of anyone else's design**. So the
  accent is isoshelf's own blue one step calmer, the greys are neutral rather
  than tinted, and the logo's two paler blues became that same colour thinned.
  What makes it read as a tool is the density and the restraint, not the
  colour.
- The change that carries it: **a status is a coloured mark beside plain
  words, not a filled chip**. Ten filled chips in one column competed with
  each other and with the names beside them. Only the two that should
  interrupt you keep a tint.
- **It is one stylesheet and no markup**, which is the good news about how
  v0.3.0 left the structure: the bones were right, only the surface needed
  work.
- Two promises are now tests rather than intentions: higher contrast puts
  every filled chip back, and the sort control is capped in em so Larger text
  can reach it. Both were checked by breaking them first.
- Also: **a 32-bit image no longer says "x86" on its badge**. The maintainer
  reported it as the catalog needing cleaning up; checking all 86 entries
  found zero disagreements between name and `arch`, so the data was right and
  the badge was wrong - it drew the catalog's own word, and "x86" reads as
  the ordinary kind. It says the bit width now, with the full name in the
  tooltip, and a test stops anything drawing the raw value as a badge again.
- Next: the tools list (TODO 16) and [#6].

### v0.4.16 (2026-09-23)

- **v0.4.16: the archive empties itself after 7, 30 or 90 days**, the item the
  maintainer picked out of decision 15. It rides the auto-update tick, because
  both questions are "has enough time passed?" and neither wants a clock.
- **It is off unless a number is chosen, and that is not a default waiting to
  be changed.** This is the only thing isoshelf does by itself that throws
  away something somebody might still want: the archive is the undo for every
  removal and every replaced file. A timer that started deleting on upgrade
  would break the rule the whole program rests on.
- Three cares, each with a test: every file is judged by its own `GoneAt`, not
  by one sweep of the folder; a file isoshelf has no archive record for is
  never deleted, however long it has sat there; and Settings says what the
  next sweep would take, in files and bytes, before it takes it.
- `update.RemoveArchived` is the hands - it deletes the names it was given and
  refuses anything that isn't a plain filename. `EmptyRemoved` still clears
  the lot, but only when somebody presses Empty.
- Next: the look half of v0.5.0 (waiting on the maintainer's yes), the tools
  list (page work, so it waits on the look too), and [#6].

### v0.4.15 (2026-09-23)

- **v0.4.15: messages shown while a dialog was open were drawn behind it.**
  The maintainer: "I clicked That's it and nothing happened." Reproduced in a
  browser, and that is exactly what it looked like - the message was there,
  underneath. A dialog opened with `showModal()` is drawn in the browser's
  top layer, above the whole page, and `showNotice` wrote into the page.
- Every refusal raised from inside a dialog was invisible: the identify
  dialog, the checklist, the folder chooser. The one that matters most is
  "a scan is running", because a NAS that checks by itself is running one
  often - so the button really did nothing, repeatably, with no way to find
  out why.
- `showNotice` now writes into the open dialog as well, and that copy is
  removed when the dialog closes so it can never come back stale. The page's
  own notice is still set, so the message survives the dialog going away.
- **The lesson worth keeping:** a page that tells people things has to know
  where the person is looking. The top layer is not a detail of styling; it
  decides whether anything said gets read at all. Anything new that reports
  through `showNotice` should be tried once with a dialog open.
- Also, from the same report: confirming the identity a file already had said
  "is now treated as" and started an online check that could only give the
  same answer. It says it was already that, and asks nobody.

### v0.4.14 (2026-09-23)

- **v0.4.14: downloads say where they come from.** The maintainer, watching
  one run: "it would be nice to know if it was from the internet or from the
  local server". Copying from another isoshelf shipped in v0.4.9 and was
  invisible while it happened - the same "Downloading 40%" either way, for a
  feature whose entire value is that one of those is much faster. The dock
  now reads "... from the NAS" or "... from the internet", and the finished
  list says "Added from the NAS".
- Nothing new is discovered to do it: `fetch.Progress` has carried the URL
  since the fetcher was written, and the web layer threw it away.
  `sourceName` (`internal/web/peer.go`) turns it into the peer's own name,
  falling back to its host, and anything else into "the internet".
- It answers a question that had no answer before: whether a peer somebody
  set up is being used at all. Short of watching a router, there was no way
  to find that out.
- The words name isoshelf, not the box: "from your isoshelf server", at the
  maintainer's asking. The folder's own name said where the bytes were
  without saying what served them, and only one peer is ever set up, so
  nothing is ambiguous for want of the address.
- **Also in v0.4.14: an Added column**, sortable like the others. The date was
  already drawn under each name; what was missing was sorting by it. On a
  phone it stays under the name, since there are no columns there.
  - It reads "-" more often than it should on Linux: `Added` falls back
    through `scan.File.Created`, `PlacedAt`, `FirstSeen`, and `created()` on
    Linux always returns zero because birthtime needs `statx`, which is not
    in `syscall` and would otherwise be a second dependency. Recorded in
    TODO item 13. A file isoshelf downloaded, or one that appeared after the
    first scan, has a date; one that was there before isoshelf ever ran does
    not.
- Next: the look half of v0.5.0, and the tools list (TODO item 16).
### v0.4.13 (2026-09-22)

- **v0.4.13: the words on the page.** The maintainer read the page and said
  the wording "sounds so strange", and named two things: "1 to download by
  hand" should say download manually, and the "By hand only" button looked
  clickable but was a disabled label. Both fixed - that button is now "Show
  them", filtering the list to the images you have to fetch yourself - and
  every other label, status, button, hint and dialog went the same way,
  toward the ordinary word: *Update available*, *End of life*, *Download
  manually*, *Unrecognized*, *Checksum mismatch*, *Check failed*. In Settings,
  *by itself* became *automatically* and *Who can get in* became *Sign-in*.
  "Room" is space, *Didn't work* is *Failed*, and checking a download against
  its checksum is called verifying, so it isn't confused with the update
  check.
  - Asked whether to keep both registers behind a "simple mode" switch, the
    answer was to default to the normal words - so that switch was not built:
    two versions of every string across ten files is a cost with no end, and
    the explanations still live in the hover and the second line, which is
    where they serve both readers.
  - Also in v0.4.13: the browser tab says **isoshelf server** on anything
    reachable from another machine, so a NAS tab and a desktop tab are not
    two tabs with the same name.
  - **The README's Roadmap was eleven releases out of date** and the
    maintainer caught it, not me: it listed server mode under "Later" (shipped
    v0.4.0) and where each folder's records live under "Next" (v0.4.2). Fixed,
    and CLAUDE.md now names those four lines as the thing to re-read at every
    release, because no test can tell that a plan has come true. A test does
    count the catalog, which is the part that can be checked.
  - This is the wording half of v0.5.0. The look is what is left.

- **v0.4.12: sign out never worked, and the reason was already written down
  three inches away.** It was a plain form posted to /login, and this server
  sends Referrer-Policy: no-referrer, so browsers send "Origin: null" on a
  form post - the exact fact that has a paragraph about it in login.go,
  written when the login form hit it. The login form got a form-token to
  work round it; sign-out, sitting next to it, kept the same-origin check and
  was refused every single time with "request refused".
  - The lesson is not about Origin. It is that a fix written for one caller
    of a rule should be followed by looking for the other callers of the
    same rule, and I wrote the paragraph and didn't.
  - Sign-out is an ordinary endpoint behind the guard now, where a fetch
    sends a real Origin. A POST to /login is a sign-in, never a sign-out,
    which is what made it possible to confuse the two in the first place.

- **v0.4.11 takes v0.4.10 straight back out, and it is worth remembering
  why.** Marking every release below 1.0 as a pre-release broke the update
  notice for every isoshelf already installed: they ask GitHub for
  `/releases/latest`, which skips pre-releases, so that call returned the
  last unmarked release forever and nobody was told a new one existed. The
  fix - reading the list instead - shipped in the release they would first
  have had to be told about.
  - The trap was spotted for the code being written and missed completely
    for the copies already running. **Changing what a published thing is
    labelled is a change to every client that already asks about it**, and
    that is the question to ask first.
  - Only a `-rc` tag is a pre-release now. The list-reading in `appupdate`
    stays: it is more robust anyway, and it is what makes a real `-rc`
    release work when there is one.

- **What the version number means, answered 2026-09-22.** The maintainer
  asked about separating pre-release, beta and stable, and about whether
  branches were the mechanism. They aren't: a branch is a workspace for
  unfinished work, a tag is a published thing, and stable-vs-pre-release is
  a label on a release. So v0.4.10 marks every release below 1.0 as a
  pre-release automatically, which GitHub already has a badge for. Their
  proposed "1.0-2.0 is beta" was talked out of rather than built: nobody
  reads 1.x as unfinished, and it would delay being able to say "stable" by
  a whole major version for no gain.
- That change had a trap in it worth recording: GitHub's "latest release"
  endpoint leaves pre-releases out, so marking them would have silently
  stopped isoshelf telling anybody a new version existed. `appupdate` now
  reads the list, and offers a pre-release only to somebody already running
  one - below 1.0, everybody.

- v0.4.9 is the maintainer's feature request: one isoshelf copies an image
  from another on the same network before going to the internet. The shape
  that made it small is that `fetch.Request.URLs` was already "official site
  first, then mirrors" - a peer is one more place, put at the front.
- **The peer is reached, never trusted**, and that is the whole security
  argument. The checksum still comes from the project's own HTTPS site, so a
  copy that is stale, damaged or served by something pretending to be an
  isoshelf fails the same check any download would. `fetch` had to learn to
  fall through to the next place on a mismatch, or one bad copy would have
  cost the whole download.
- Files are asked for **by hash, not by name**: a name alone says yes to a
  stale copy, which for a fixed-name image is exactly the file being
  replaced. That meant sharing has to hash everything, not only fixed-name
  images - so turning sharing on changes what a scan does, and says so.
- The sharing machine records which drives took what, which the maintainer
  asked for and which is most of what #11 (rebuild a lost drive) needs.
- **Still to decide: finding the other isoshelf by itself.** Doing it
  properly means mDNS, which means a second dependency or a good deal of
  protocol code - a decision for the maintainer, not an omission. The
  address is typed in until then.
- Next, at the maintainer's pick: emptying the archive on a timer.

- v0.4.8, both from the maintainer using it: Settings saved silently, and it
  was too wordy. A "Saved" mark now appears beside the setting that changed -
  each entry names the answer it owns, so the mark lands on the right row -
  and every hint is cut to a sentence or two with the small print moved to
  the note under the control. Two tests hold both: one fails if a setting
  saves an answer no entry claims, the other if a hint goes over 210
  characters. The longest was 463.

- v0.4.7, both halves from the maintainer's own install. **The link's secret
  now stops working once a password is set** - their ask, outright: "I want
  to get rid of tokens moving forward and only have a login screen." It still
  works before one is set, or a fresh install couldn't be opened; after that
  it is nothing, cookie included, with no restart needed. The way back from a
  forgotten password is `isoshelf password` from a shell or the two
  environment variables - both need the machine itself, neither is a second
  door on the network.
- **And the permission check had a hole their bug reports found.** Three
  reports, all "open /images/.isoshelf/partial/...: permission denied". The
  startup check tested the images folder and not isoshelf's own folder inside
  it - which is older than the current arrangement, made by whichever user
  isoshelf ran as before, and left behind when the app's user changed. So the
  folder tested fine and every download failed. Reproduced as uid 568 against
  a folder owned 568 with a .isoshelf owned 1000; the warning now names the
  folder, its owner, how it got that way, and the chown - including that
  changing the app's user instead is the wrong answer, because it would break
  the folder that works.

- The maintainer's own install turned up three things, all shipped as v0.4.4.
  A folder isoshelf can't write to produced a wall of "permission denied"
  from whichever part wrote first and said nothing about what to do; it now
  checks both folders at startup and names the folder, the user it runs as
  and the two ways to fix it on TrueNAS. Reporting a problem opened an empty
  GitHub form, because GitHub's phone app ignores anything filled in from a
  link; the dialog now shows the report as one block with a Copy button, with
  a fallback for plain http where the browser's clipboard doesn't exist. And
  the question dialog had no height limit, so on a phone its buttons were
  below the bottom edge with no way to scroll to them.
- v0.4.6 is updating by itself, the second of the two. On a schedule it
  checks and puts everything it finds through the same queue the Update
  button uses. Off unless turned on, and it will stay that way - it is the
  one thing isoshelf does that changes a drive unattended. Building it turned
  up a real bug of its own: the settings file had one writer and now has two,
  so a switch and the scheduler noting the time could land on each other and
  lose one of the changes. Every write to that file now takes a turn.
- Both questions answered by the maintainer, 2026-09-22. **A username and
  password**, over keeping the token alone or an off switch - shipped as
  v0.4.5. And **updating ISOs fully automatically**, end to end: check,
  download, verify and put in place, each image following the answer it
  already carries (replace / archive / keep both). That is next.
- v0.4.5 notes worth keeping: the login form cannot use the Origin check the
  rest of the page uses, because isoshelf sends Referrer-Policy: no-referrer
  and browsers then send "Origin: null" on a plain form post - it carries a
  SameSite=Strict cookie and a matching hidden field instead. Sessions are
  signed rather than remembered, so a NAS app update doesn't sign anyone out.
  And the login page's inline stylesheet needs its own hash in the content
  policy, or it arrives as unstyled HTML.

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
- v0.4.3 finishes decision 10: the folder chooser lists the folders isoshelf
  remembers with when each was last used, what it held and Forget. All of it
  comes from the copies in isoshelf's own folder - a folder on that list may
  be a NAS that is asleep, and opening the chooser must not go looking for
  it. Forgetting removes that copy and the folder's records answer, and
  nothing else.
- Next: decision 15's leftovers (empty the archive after so many days, a
  download speed limit, hiding kinds and architectures), or the v0.5.0 page
  redesign, which is the maintainer's own ask - "something I could show an
  investor" - and wants their answers to the seven questions in
  `docs/design-audit.md` before it starts.

### Earlier (2026-09-21)

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
  updates; Settings gained the automatic update check and the new-version
  notice (which was a command-line flag only). Failures are
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

[#1]: https://github.com/ZachCurry13/isoshelf/issues/1
[#2]: https://github.com/ZachCurry13/isoshelf/issues/2
[#3]: https://github.com/ZachCurry13/isoshelf/issues/3
[#4]: https://github.com/ZachCurry13/isoshelf/issues/4
[#5]: https://github.com/ZachCurry13/isoshelf/issues/5
[#6]: https://github.com/ZachCurry13/isoshelf/issues/6
[#7]: https://github.com/ZachCurry13/isoshelf/issues/7
[#13]: https://github.com/ZachCurry13/isoshelf/pull/13
[#14]: https://github.com/ZachCurry13/isoshelf/pull/14
[#55]: https://github.com/ZachCurry13/isoshelf/issues/55
[#54]: https://github.com/ZachCurry13/isoshelf/issues/54
[#59]: https://github.com/ZachCurry13/isoshelf/issues/59
[#62]: https://github.com/ZachCurry13/isoshelf/issues/62
[#56]: https://github.com/ZachCurry13/isoshelf/issues/56
[#58]: https://github.com/ZachCurry13/isoshelf/issues/58
[#57]: https://github.com/ZachCurry13/isoshelf/issues/57
