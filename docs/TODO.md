# What's next

The running work list. A new session should read this first, then
`CLAUDE.md` for the rules and `docs/design.md` for the why. `docs/STATUS.md`
says where things stand and what was decided; this file says what to do.

Keep it current: tick an item off when it ships, and add what you learn while
doing it, so the next session doesn't rediscover it.

## Right now

**Nothing is in flight.** `main` is at `c49b13a`. v0.3.4 is written and
pushed but unreleased: release it the way v0.3.3 was released (the workflow
started by hand from the Actions tab with the version typed in, since the
sandbox refuses `v*` tag pushes with a 403).

**v0.3.3 is the latest release** (published 2026-09-21), and there are twelve
releases going back to v0.2.0, the first one. There is no v0.3.1 or v0.3.2
release: both versions have their own `CHANGELOG.md` section, but they went
out inside the v0.3.3 release, because a release happens when a `v*` tag is
pushed and those two were never tagged. Note for anyone checking this the way
it was got wrong once: this is a shallow clone with no tags fetched, so
`git tag` prints nothing even though releases exist. Ask GitHub, not the
clone.

1. ~~Merge [#13]~~ *(done: merged as `9f609a6`, with the release-workflow fix
   [#14] as `c49b13a`; both branches are deleted)*.
2. ~~Close issue #10~~ *(done: it shipped in v0.3.0 and was left open)*.
3. ~~Release v0.3.3~~ *(done: the five versioned files came out right, with
   the plain names still inside the portable zip)*.

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
   drive ([#7]) - **and the redesign below**.

## v0.4.0: the page, redone

The maintainer's brief, given 2026-09-21: **"clean up the UI - make it look
like something I could show an investor."** Not a priority before then; the
0.3.x items above come first. What was said when asked what drives it: the
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

`docs/design-audit.md` is a critique of the page written before any of this
was built, with seven such questions already worked out and a staged plan.
Start there; it is a proposal, not a decision.

It also turned up **two live bugs**, small enough to fix in a 0.3.x release
rather than wait:

- The Filter menu runs off the **left** edge at phone width (390px).
  `placeMenu` clamps against the right edge only.
- **Escape doesn't close the details panel's "…" menu.** Settings, the
  details panel and the dock are wired to Escape; `details.menu` isn't.

## Worth knowing before you start

- **The page is eight scripts**, not one. `app.js` was 2,544 lines until
  v0.3.3; it is now `app.js` (state, asking, drawing, wiring), `images.js`,
  `details.js`, `downloads.js`, `actions.js`, `folders.js`, `archive.js` and
  `settings.js`. Read the one you need. A new one goes in `index.html` and in
  `scripts` in `internal/web/static_test.go`.
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
