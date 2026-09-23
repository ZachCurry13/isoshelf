# What's next

The running work list. A new session should read this first, then
`CLAUDE.md` for the rules and `docs/design.md` for the why. `docs/STATUS.md`
says where things stand and what was decided; this file says what to do.

Keep it current: tick an item off when it ships, and add what you learn while
doing it, so the next session doesn't rediscover it.

## Right now

**In flight: v0.5.0.** Its wording half shipped as v0.4.13.

**What v0.5.0 holds, decided 2026-09-23** (the maintainer: "I want to make
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
5. **Moving a folder's records asks first.** Changing "Where this folder's
   records are kept" relocates data the instant the dropdown changes, with no
   warning and no result afterwards. What it does is safe - the new file is
   written before the old one is removed, a destination that already has
   records is left alone, and the archive never moves - but the maintainer
   had to ask what it did, which is the bug. Say what will move, from where
   to where, before doing it, and say what happened after, including the
   silent "both existed so I kept both".

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

   **Not in v0.5.0**, which is polish and correctness only. This changes what
   "verified" means at the edges and deserves a release where the
   unverified-copy rules get real tests, rather than riding along in one whose
   whole point is that nothing in it is half-built. First thing after, ahead
   of [#1] and [#4]: it is what makes running the server worth it.

**After v0.5.0, in this order:** item 18 (asking the server by entry), [#1]
the command line, [#4] two downloads at once, [#2] older versions with a hold, [#3] make bootable (rename and extract
only - decided 2026-09-23), [#5] OpenPGP signatures (the second dependency is
accepted - decided 2026-09-23), then [#11] and [#12].

Everything else below the next heading has shipped.

**Which version is the latest is not written down here, on purpose.** It went
stale three times in one evening. Ask GitHub: the releases page, or
`gh release list`. This is a shallow clone with no tags fetched, so `git tag`
prints nothing even though releases exist - that has caught two sessions out
now, so ask GitHub rather than the clone.

Worth knowing about the numbering: there is no v0.3.1 or v0.3.2 release.
Both have their own `CHANGELOG.md` section but went out inside v0.3.3,
because a release happens when a `v*` tag is pushed and those two were never
tagged. Don't let it happen again - one version, one tag, one release.

**How a release happens:** merge the pull request into `main`, then push a
`v*` tag (or start the `release` workflow from the Actions tab with the
version typed in). Either way it builds three binaries and the portable zip,
names them with the version, attaches `SHA256SUMS`, and takes the notes from
that version's `CHANGELOG.md` section. No section, no release.

Then start the **container image** workflow with the same version typed in.
It does not follow from the release: a tag created by one workflow
deliberately doesn't set another off, so the release workflow's tag never
reaches it. A tag pushed by a person does.

## ~~v0.3.4: scanning while downloads run~~ *(done)*

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

## Answered 2026-09-21, late

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

## 1.0: in the TrueNAS store

The maintainer asked to try for this. What it actually takes, so nobody
starts it thinking it is a form to fill in:

- **A published image that keeps working.** TrueNAS's catalog points at a
  registry; if a tag moves or breaks, it breaks for everyone who installed
  it. That is the real commitment, and it does not end when the pull request
  is merged.
- **An entry in `truenas/apps`**, in `ix-dev/community/isoshelf/`. Started:
  `deploy/truenas/` holds `app.yaml`, `item.yaml` and `ix_values.yaml`,
  written against the real schema of an existing community app rather than
  guessed. Its README says what is still missing and why - chiefly
  `questions.yaml` and the compose template, which call TrueNAS's own
  template library and can't be written faithfully without reading one of
  theirs in full.
- **An icon.** A real one, not a letter in a box.
- **Sensible defaults for someone who has never seen isoshelf.** The install
  form has to make the token, not ask for one - a person pasting `password`
  into that box has published their images folder to their whole network.
- **Worth knowing, and checked rather than assumed:** the folder chooser does
  work in a container. It browses the container's filesystem, which is where
  the mounts are, so a second folder is a second mount and both then show up
  in **Choose folder…**. What it shows is the container's path (`/images`),
  not the host's (`/mnt/tank/isos`), which is worth one line in the app's
  description so nobody goes looking for a path that isn't there.

## Then, in order

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
10. **isoshelf updates itself** (decisions 13 and 14): waits for downloads,
   swaps its own program, restarts, page reconnects - and only installs a
   release carrying the project's signature, which needs the signing key set
   up once. Whatever downloads the new file must find its asset **by pattern**
   (the name contains `windows-amd64.exe`), never by an exact name: release
   files carry the version now, so an exact name goes stale every release.
11. **Rebuild a drive** ([#11]) and **move a drive to a bigger one** ([#12]).
   Same feature, two reasons for wanting it, both from real r/Ventoy posts
   where people lost a drive's worth of images. The list is already kept off
   the drive: `internal/state/mirror.go` copies each folder's state into the
   config folder, including the entry ids seen by each scan. What's missing is
   the verb, plus exporting the list to a file someone can keep elsewhere.
12. **Empty the archive after so many days** (decision 15, and the
   maintainer picked this one next, 2026-09-22). 7 / 30 / 90 days or never,
   in Settings. Care needed: the archive is the undo for every removal and
   every replaced file, so emptying it on a timer is the one thing here that
   throws away something somebody might still want. It has to say what it
   will do before it does it, count from when each file was archived rather
   than sweeping the lot, and never touch a file archived since the last
   scan. The rest of decision 15 - a download speed limit, hiding kinds and
   architectures you don't use - can follow.
13. **From the maintainer's own use, 2026-09-22**
   (`isoshelf_v0.4.9_tasks.md`; numbered v0.4.9 there, but that number went
   to copying between isoshelfs, so these are next rather than done):
   - **Hand-copied images are still called "to do by hand".** Copy an ISO
     onto the drive yourself and isoshelf keeps saying an update is waiting.
     Work out why the scan doesn't match it to the entry it plainly is -
     start with `internal/identify` and the assignment path, and with what
     `check.Manual` actually means today.
   - ~~**Say when a file arrived on the drive.**~~ *(done in v0.4.14: an
     Added column in the list, sortable, and still under the name on a
     phone. The date was already drawn under each name - what was missing
     was being able to sort by it.)*
     - **What is still missing is the date itself, on Linux.** `Added` is
       `scan.File.Created`, then `PlacedAt`, then `FirstSeen`, and on Linux
       `created()` always returns zero (`internal/scan/created_other.go`):
       birthtime needs `statx`, which is not in the `syscall` package and
       would otherwise mean a second dependency. So a file that was on the
       drive before isoshelf first scanned it reads "-" forever. Worth doing
       with a hand-rolled `statx` call if it can be done without the
       dependency and without per-architecture syscall numbers going stale;
       not worth guessing a date for.
   - **Say when the new version came out.** An update says a newer version
     exists but not its age, which is most of deciding whether to take it.
     The release date is in what `source` already fetches for some sources;
     check which, and show it where it is known.
   - **The filter menu needs Apply and Clear all.** Filters take effect as
     they are ticked, which leaves people unsure anything happened. Either
     add the buttons or make "it already applied" obvious - worth deciding
     rather than assuming, and it overlaps the redesign below.
   - **Offer to send an unknown image to the catalog.** When somebody names
     an image isoshelf doesn't recognize, offer to open a prefilled issue
     with the filename pattern and hash - the same shape as the bug report
     in `report.js`, pointed at the missing-image form. Nothing is sent
     without them pressing submit.
14. **The rest of decision 15**: a download speed limit, hiding kinds and
   architectures you don't use.
15. **v0.4 proper**: `isoshelf update` on the command line ([#1]), installing
   an older version with a hold ([#2]), Make bootable ([#3]), two downloads at
   once ([#4]), OpenPGP signatures ([#5]), the portable zip tried on a real
   drive ([#7]) - **and the redesign below**.
16. **The tools that go with the images** (the maintainer, 2026-09-22:
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

## v0.5.0: the page, redone

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

## Worth knowing before you start

- **The page is one script per part**, not one script. `app.js` was 2,544
  lines until v0.3.1; it is now `app.js` (state, asking, drawing, wiring),
  `images.js`, `details.js`, `downloads.js`, `actions.js`, `folders.js`,
  `archive.js`, `settings.js`, `upload.js` and `report.js`. Read the one you
  need. A new one goes in `index.html`, in `scripts` in
  `internal/web/static_test.go`, and in every list of them - which
  `internal/docs` checks, because this list was stale for four releases.
- **`deadcode` comes back clean** and should stay that way:
  `GOTOOLCHAIN=go1.27.1 go run golang.org/x/tools/cmd/deadcode@latest -test ./...`.
  `staticcheck` is not clean and hasn't been: it flags
  `internal/web/target.go:45` for a capitalized error ending in a full stop
  (ST1005). That one is deliberate - the string is shown to the user as a
  sentence - so the finding stays. Check new findings against that.
- **A full drive shows bugs a small folder hides.** Build one from the
  filenames in `internal/sampledrive` (93 of them) before judging the page:
  `go run ./internal/sampledrive/mkdrive <folder>` writes them all as
  stand-ins. That is how the filter menu bug was found.
- **A machine with no internet shows "Couldn't check" on every row.** That is
  the network, not a bug. The recorded responses in the tests are the way to
  check that path without going online.
- **Don't touch port 8765**: the maintainer's own preview runs there.
- **Before changing what a release is labelled, work out what the copies
  already installed will ask for.** v0.4.10 marked every `v0.*` as a
  pre-release; GitHub leaves those out of `/releases/latest`, which is what
  every isoshelf already installed asks - so the update notice went silent
  everywhere, and the fix was in the release nobody could be told about.
  Taken back out in v0.4.11: only a `-rc` tag is a pre-release. Link to
  `/releases` rather than `/releases/latest` all the same, so a future `-rc`
  never hides the newest finished release.
- **Release files carry the version** since v0.3.3, so a link straight to
  `releases/latest/download/isoshelf-windows-amd64.exe` no longer works.
  Nothing in this repository used one and the published downloads had been
  taken a handful of times at most, so it was the cheapest possible moment
  - but it was a break, not a free change, and it shouldn't happen twice.

## Questions the maintainer still owes an answer to

- ~~*Keep both* in the name-clash dialog~~ *(answered in v0.3.6, and the old
  answer was wrong: it can work. The new download carries its version in its
  name and the file already on the drive is not touched - the maintainer
  chose that direction in v0.3.7, over renaming the old file, because then
  nothing that exists is disturbed. See `internal/update/keepboth.go`.)*
  Still open:
  an explicit *Cancel*, which is currently "do nothing" plus a message saying
  nothing has changed. Add a real Cancel button that clears the row?
- Anything from the review doc listed as done that doesn't feel done when
  used on a real drive.

[#13]: https://github.com/ZachCurry13/isoshelf/pull/13
[#14]: https://github.com/ZachCurry13/isoshelf/pull/14
[#1]: https://github.com/ZachCurry13/isoshelf/issues/1
[#2]: https://github.com/ZachCurry13/isoshelf/issues/2
[#3]: https://github.com/ZachCurry13/isoshelf/issues/3
[#4]: https://github.com/ZachCurry13/isoshelf/issues/4
[#5]: https://github.com/ZachCurry13/isoshelf/issues/5
[#6]: https://github.com/ZachCurry13/isoshelf/issues/6
[#7]: https://github.com/ZachCurry13/isoshelf/issues/7
[#11]: https://github.com/ZachCurry13/isoshelf/issues/11
[#12]: https://github.com/ZachCurry13/isoshelf/issues/12
