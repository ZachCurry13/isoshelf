# What's next

The running work list. A new session should read this first, then
`CLAUDE.md` for the rules and `docs/design.md` for the why. `docs/STATUS.md`
says where things stand and what was decided; this file says what to do.

Keep it current: tick an item off when it ships, and add what you learn while
doing it, so the next session doesn't rediscover it.

## Right now

**Nothing is in flight.** `main` is at `c49b13a` and carries everything
through v0.3.3, so the next piece of work branches from there, named after
the work (see CLAUDE.md).

**v0.3.3 is the latest release** (published 2026-09-21), and there are twelve
releases going back to v0.2.0, the first one. There is no v0.3.1 or v0.3.2
release: both versions have their own `CHANGELOG.md` section, but they went
out inside the v0.3.3 release, because a release happens when a `v*` tag is
pushed and those two were never tagged. Note for anyone checking this the way
it was got wrong once: this is a shallow clone with no tags fetched, so
`git tag` prints nothing even though releases exist. Ask GitHub, not the
clone.

The three things that stood here are all done:

1. ~~Merge [#13]~~ *(done: merged as `9f609a6` - v0.3.1 (Settings), the
   dead-code clear-out and the `app.js` split, the documentation pass, v0.3.2
   (checking by itself), v0.3.3 (fixes and tidying), the versioned release
   filenames, and the agents. The release-workflow fix followed in [#14] as
   `c49b13a`. Both branches are deleted.)*
2. ~~Close issue #10~~ *(done: it shipped in v0.3.0 and was left open)*.
3. ~~Release v0.3.3~~ *(done: pushing a `v*` tag is the normal way, but the
   sandbox blocks tag pushes with a 403, so the release workflow was started
   by hand from the Actions tab with the version typed in - that creates the
   tag and the release itself. The first release with versioned asset names
   came out right: `isoshelf-v0.3.3-windows-amd64.exe`,
   `isoshelf-v0.3.3-linux-amd64`, `isoshelf-v0.3.3-linux-arm64`,
   `isoshelf-v0.3.3-portable.zip` and `SHA256SUMS`, with the plain names
   still inside the zip and the notes taken from that version's
   `CHANGELOG.md` section.)*

So the next piece of work is v0.3.4, below.

## v0.3.4: scanning while downloads run

The last real bug from the maintainer's review. Today a scan or Refresh is
refused while anything is downloading (`busyLocked` in
`internal/web/queue.go`), which locks the page for as long as a queue takes.

What it needs, and why it isn't a five-minute change:

- The server has **one** `s.run` slot that a scan and a download share, and a
  download is "a run whose `job` is set". Two things at once means two slots.
- The page reads the running download's progress from `state.run`
  (`downloadProgress()` in `static/downloads.js`), so splitting the slot
  changes the JSON the page reads. The scan's own progress card (`#run`) and
  the downloads dock are already separate on screen, so the page can show
  both - `downloadsJSON.Current` needs the progress fields the dock uses.
- A scan finishing mid-download replaces `s.report`, `s.st` and `s.scan`
  wholesale (`execute` in `scanrun.go`). A file placed while the scan ran
  would be missing from that copy until the next scan. The scan that already
  runs when the queue drains (`startNextLocked`) heals it, but check that.
- The state **file** is already safe: each writer keeps the copy from before
  its change and `state.Merge` carries only that change onto what is on disk
  (`saveStateLocked`). Don't undo that.
- Switching folders should still wait for downloads. That part of the lock is
  right, and the maintainer agrees.

Keep this one in hand rather than delegating it: it is architecture, and it
sits next to the state file.

## Then, in order

4. **Upload from the browser** (from the review doc). Drag a file onto the
   page, or pick one, and it lands in the folder. Its own step, because it
   writes to the drive: image files only, inside the chosen folder, nothing
   overwritten without asking, and the same "what is this file?" pass
   afterwards that a copied-in file gets.
5. **Where each folder's records live** (decision 10 in STATUS.md). The
   fields are already in `internal/settings` (`StateLocation`, `StateDir`,
   `InFolder`/`WithApp`/`Elsewhere`); nothing reads them yet, and the method
   that used to turn them into a path was removed as dead code - write it
   again when you build this. Plus the remembered-folders list with last
   used, size and Forget.
6. **isoshelf updates itself** (decisions 13 and 14): waits for downloads,
   swaps its own program, restarts, page reconnects - and only installs a
   release carrying the project's signature, which needs the signing key set
   up once. Whatever downloads the new file must find its asset **by pattern**
   (the name contains `windows-amd64.exe`), never by an exact name: release
   files carry the version now, so an exact name goes stale every release.
7. **Rebuild a drive** ([#11]) and **move a drive to a bigger one** ([#12]).
   Same feature, two reasons for wanting it, both from real r/Ventoy posts
   where people lost a drive's worth of images. The list is already kept off
   the drive: `internal/state/mirror.go` copies each folder's state into the
   config folder, including the entry ids seen by each scan. What's missing is
   the verb, plus exporting the list to a file someone can keep elsewhere.
8. **The rest of decision 15**: empty the archive after 7/30/90 days, a
   download speed limit, hiding kinds and architectures you don't use.
   Settings has a home for all of them now.
9. **v0.4 proper**: `isoshelf update` on the command line ([#1]), installing
   an older version with a hold ([#2]), Make bootable ([#3]), two downloads at
   once ([#4]), OpenPGP signatures ([#5]), the portable zip tried on a real
   drive ([#7]).

## Worth knowing before you start

- **The page is eight scripts**, not one. `app.js` was 2,544 lines until
  v0.3.3; it is now `app.js` (state, asking, drawing, wiring), `images.js`,
  `details.js`, `downloads.js`, `actions.js`, `folders.js`, `archive.js` and
  `settings.js`. Read the one you need. A new one goes in `index.html` and in
  `scripts` in `internal/web/static_test.go`.
- **Two dead-code checkers come back clean** and should stay that way:
  `GOTOOLCHAIN=go1.27.1 go run golang.org/x/tools/cmd/deadcode@latest -test ./...`
  and `staticcheck`.
- **A full drive shows bugs a small folder hides.** Build one from the
  filenames in `internal/sampledrive` (93 of them) before judging the page.
  That is how the filter menu bug was found.
- **The sandbox can't reach the image projects' sites**, so a preview here
  shows "Couldn't check" on every row. That is the sandbox, not a bug. The
  recorded responses in tests are the way to check that path.
- **Don't touch port 8765**: the maintainer's own preview runs there.
- **Release files carry the version** since v0.3.3, so a link straight to
  `releases/latest/download/isoshelf-windows-amd64.exe` no longer works.
  Nothing in this repository used one and the published downloads had been
  taken a handful of times at most, so it was the cheapest possible moment
  - but it was a break, not a free change, and it shouldn't happen twice.

## Questions the maintainer still owes an answer to

- The name-clash dialog offers *Archive the old one* and *Replace it*. The
  review doc also asked for *Keep both*, which cannot work for these images
  (the clash only happens when the filename never changes), and an explicit
  *Cancel*, which is currently "do nothing" plus a message saying nothing has
  changed. Add a real Cancel button that clears the row, or leave it?
- Anything from the review doc listed as done that doesn't feel done when
  used on a real drive.

[#13]: https://github.com/ZachCurry13/isoshelf/pull/13
[#14]: https://github.com/ZachCurry13/isoshelf/pull/14
[#1]: https://github.com/ZachCurry13/isoshelf/issues/1
[#2]: https://github.com/ZachCurry13/isoshelf/issues/2
[#3]: https://github.com/ZachCurry13/isoshelf/issues/3
[#4]: https://github.com/ZachCurry13/isoshelf/issues/4
[#5]: https://github.com/ZachCurry13/isoshelf/issues/5
[#7]: https://github.com/ZachCurry13/isoshelf/issues/7
[#11]: https://github.com/ZachCurry13/isoshelf/issues/11
[#12]: https://github.com/ZachCurry13/isoshelf/issues/12
