# What's next

The running work list. A new session should read this first, then
`CLAUDE.md` for the rules and `docs/design.md` for the why. `docs/STATUS.md`
says where things stand and what was decided; this file says what to do.

Keep it current: when an item ships, move it (with what was learned doing it)
to `docs/archive.md`, so this file stays a list of what is left. Items keep
their numbers when others move out, because other items and decisions refer
to them by number; a gap in the numbering is an item in the archive.

## Right now

**Next: item 3, missing images ([#55]), then item 18 ([#62]).** Then the
rest of the maintainer's queue, then the list under *After those*. v0.7.0,
the simpler shelf, has shipped; it is in `docs/archive.md`.

**Left over from v0.5.0:** [#6], the Fedora entries stop pinning a release
number. This is the least 1.0 thing in the repository: when Fedora 45 ships,
isoshelf keeps offering 44 and nothing fails, which is the kind of quiet
wrongness that costs trust once somebody notices.

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

   **Not in v0.5.0**, which is polish and correctness only. This changes what
   "verified" means at the edges and deserves a release where the
   unverified-copy rules get real tests, rather than riding along in one whose
   whole point is that nothing in it is half-built. First thing after, ahead
   of [#1] and [#4]: it is what makes running the server worth it.

19. **Default credentials, where the project publishes them** (the
   maintainer, 2026-09-23: "if any of these systems have default passwords
   like below, that should be noted - user demo: demo, user root: root").
   That example is MX Linux's documented live-session default, so the case is
   real: a rescue or live image you boot once a year is exactly the thing
   whose login you will not remember.

   **The rule, which matters more than the field.** Credentials come from the
   project's own documentation and nowhere else - the same rule as checksums,
   and for the same reason. Not a forum, not a wiki somebody else runs, not
   memory. If a project doesn't document them, isoshelf says nothing; a
   password isoshelf guessed at is worse than no password at all, because
   somebody will type it into a machine they care about.

   **It rots, so it carries its source.** Defaults change between releases.
   The entry stores the address it was read from and the page shows it, so a
   reader can check rather than trust. This is the "don't write down anything
   that has to be maintained" rule bending as far as it goes: the fact is
   worth having, so it has to come with the means to re-check it.

   **Shape.** A `[entry.credentials]` table: `user`, `password`, `note` (for
   "no password", "sudo without one", "asked on first boot"), and `source`.
   Shown in the details panel, not on the row - it is worth having, not worth
   a column. Worded as what it is: "the project documents these as the live
   session's defaults", never as something isoshelf found out.

   **Not started, and not startable from these sessions.** Filling it means
   reading each project's own documentation, and every such site tested from
   here is refused by the egress proxy (see CLAUDE.md). This belongs to the
   weekly catalog job, which runs where the network works. Build the field
   and the display when there is data to put in them; an empty field shows
   nothing and helps nobody.

**Queued by the maintainer, 2026-09-23, after v0.6.0, in this order:**

3. **Missing images get two actions:** "Download again" as the row's button
   ("Download page" for images fetched by hand), and "Stop expecting it" in
   the details panel - off the missing list until the image is in the folder
   again. Filtering to the missing ones offers "Download all" when
   isoshelf can download them, the way filtering to older versions offers
   Review and clear. A missing image whose file is still in the archive
   should offer Restore first: it is instant and gives back the very file. Today a missing image offers only Details, and stays missing until
   it drops out of the last 10 scans.

   **Then item 18, [#62]** (decided 2026-09-24): getting any image from the
   server, with a record of where each file came from.
4. **Dismiss an update** for 7, 30 or 90 days or forever - **any** update,
   manual or downloadable; for a downloadable one, "forever" is the "Never
   update" list decided 2026-09-19, and Update all and automatic updates skip
   it. **The timer holds** even if a newer version comes out. The row stays
   listed, greyed, "Dismissed until 23 Oct", not counted on the updates card
   and sorted with the up-to-date ones. Settings lists **everything**
   dismissed, with Undo on each and Undo all.
5. **A "where to find it" note for images fetched by hand**, e.g. "The
   32-bit ISO is MX-{version}_386.iso, in the Xfce folder." Needs a new
   catalog field, and the catalog is read with unknown fields refused
   (`DisallowUnknownFields`), so isoshelf has to learn the field in one
   release before `default.toml` may use it - otherwise every older copy
   refuses the updated catalog. Show the maintainer where it would appear
   before building it.
6. **Note duplicate images, and offer to delete a copy.** isoshelf already
   finds *older* copies of an image; the same image and version twice, under
   two names or in two folders, isn't found. Same entry and version and size
   is the cheap first look; telling for sure means hashing both, minutes on
   a USB stick - **only when asked** (the maintainer, 2026-09-24: "I like the
   idea of verifying optionally. A lot of times I just trust it but maybe you
   can say that it needs to be verified"). So same-size copies are shown as
   *possible* duplicates, with a button to make sure. Deleting goes through
   the archive like every other removal.
7. **A sharing disclosure**, in the README and beside the sharing switch:
   the licenses of the images you share are yours to mind. The maintainer's
   view, 2026-09-23, and it holds for files people bring themselves - "like a
   Plex server". The project's own responsibility is what its catalog lists
   and what it promotes; sharing proprietary images shouldn't be pitched as a
   feature.
8. **AtlasOS: recognized, not listed.** "AtlasOS (archival ISO)" is in the
   built-in catalog with a logo, so it appears under Add images for everyone.
   AtlasOS itself moved from handing out modified Windows images to a
   playbook applied to your own Windows. Keep recognizing the file; stop
   listing it as something to go and get.
9. **Logos by trademark policy.** A project being open source doesn't make
   its logo free to use: Debian publishes an open-use logo, Canonical's
   Ubuntu policy is strict. Keep a logo only where the project's own policy
   allows this use, and a generic disc icon otherwise - starting with
   Windows, Windows 11, Ubuntu, AtlasOS and NiceHash - including the ones
   fetched from the Simple Icons CDN. The weekly catalog job can check each
   policy on the project's own site. Delisting on request already stands.

**Around 1.0, not before:** signing the Windows program so the "unknown
publisher" warning goes (the maintainer, 2026-09-23: GitHub users are used to
it). SignPath Foundation signs open-source projects for free; Microsoft's
Trusted Signing is about $10 a month; a certificate authority costs hundreds a
year and needs a hardware key. Even signed, Windows warns until the program
has a reputation. Self-update already means the warning appears only on the
first download, because files isoshelf downloads itself don't carry the
browser's "came from the internet" mark.

**After those, in this order:** [#1] the command line, [#4] two downloads at once, [#2] older versions with a hold, [#3] make bootable (rename and extract
only - decided 2026-09-23), [#5] OpenPGP signatures (the second dependency is
accepted - decided 2026-09-23), then [#11] and [#12]. Any time a real
USB drive is to hand: [#7], trying the portable zip on one.

On [#3]: the maintainer asked again for a one-click fix for a file that only
needs renaming, and chose on 2026-09-23 to keep it in this order. Until it is
built, the won't-boot note says what to do by hand (`notBootableNote`); when
it is, the note is where the button's words come from, and the catalog's
`fixup` field (`rename:.img`, `extract`) already says which fix each image
needs.

## Releasing

**Which version is the latest is not written down here, on purpose.** It went
stale three times in one evening. Ask GitHub: the releases page, or
`gh release list`. This is a shallow clone with no tags fetched, so `git tag`
prints nothing even though releases exist - that has caught two sessions out
now, so ask GitHub rather than the clone.

Worth knowing about the numbering: there is no v0.3.1 or v0.3.2 release.
Both have their own `CHANGELOG.md` section but went out inside v0.3.3,
because a release happens when a `v*` tag is pushed and those two were never
tagged. Don't let it happen again - one version, one tag, one release.

**Before every release, the public pages** (the maintainer, 2026-09-23: "you
promised you'd keep all of GitHub up to date. everything!" - after the
README's roadmap had said "Next" about work already shipped, and missed four
features, when it was handed to another assistant as the description of
isoshelf). Read, against the code, not from memory: the README's feature
list, Safety first, roadmap (the released line added, **Next** moved on) and
install steps; design.md's version plan; docs/docker.md; SECURITY.md's
promises. Open or update an issue for anything newly planned, close the ones
that shipped, and link them from the roadmap. A pull request isn't finished
until what it changes is true on GitHub's `main` too.

**How a release happens:** merge the pull request into `main`, then push a
`v*` tag (or start the `release` workflow from the Actions tab with the
version typed in). Either way it builds three binaries and the portable zip,
names them with the version, attaches `SHA256SUMS`, and takes the notes from
that version's `CHANGELOG.md` section. No section, no release.

Then start the **container image** workflow with the same version typed in.
It does not follow from the release: a tag created by one workflow
deliberately doesn't set another off, so the release workflow's tag never
reaches it. A tag pushed by a person does.

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
  form must not ask for a token - isoshelf makes its own, and a person
  pasting `password` into that box has published their images folder to
  their whole network. The username and password are optional fields;
  `deploy/truenas/README.md` says why.
- **Worth knowing, and checked rather than assumed:** the folder chooser does
  work in a container. It browses the container's filesystem, which is where
  the mounts are, so a second folder is a second mount and both then show up
  in **Choose folder…**. What it shows is the container's path (`/images`),
  not the host's (`/mnt/tank/isos`), which is worth one line in the app's
  description so nobody goes looking for a path that isn't there.

## Then, in order

These are older items, kept under the numbers they were given.

11. **Rebuild a drive** ([#11]) and **move a drive to a bigger one** ([#12]).
   Same feature, two reasons for wanting it, both from real r/Ventoy posts
   where people lost a drive's worth of images. The list is already kept off
   the drive: `internal/state/mirror.go` copies each folder's state into the
   config folder, including the entry ids seen by each scan. What's missing is
   the verb, plus exporting the list to a file someone can keep elsewhere.

Then:

13. **From the maintainer's own use, 2026-09-22**
   (`isoshelf_v0.4.9_tasks.md`; numbered v0.4.9 there, but that number went
   to copying between isoshelfs, so these are next rather than done):
   - **Hand-copied images are still called "to do by hand".** Copy an ISO
     onto the drive yourself and isoshelf keeps saying an update is waiting.
     Work out why the scan doesn't match it to the entry it plainly is -
     start with `internal/identify` and the assignment path, and with what
     `check.Manual` actually means today.
   - **The date a file arrived, on Linux.** The Added column (v0.4.14) sorts
     by `scan.File.Created`, then `PlacedAt`, then `FirstSeen`, and on Linux
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
   - **Offer to send an unknown image to the catalog.** When somebody names
     an image isoshelf doesn't recognize, offer to open a prefilled issue
     with the filename pattern and hash - the same shape as the bug report
     in `report.js`, pointed at the missing-image form. Nothing is sent
     without them pressing submit.
14. **The rest of decision 15**: a download speed limit, hiding kinds and
   architectures you don't use.

## Worth knowing before you start

- **The page is one script per part**, not one script. `app.js` was 2,544
  lines until v0.3.1; it is now `app.js` (state, asking, drawing, wiring),
  `images.js`, `summary.js`, `details.js`, `checklist.js`, `downloads.js`,
  `actions.js`, `catalog.js`, `identify.js`, `folders.js`,
  `archive.js`, `settings.js`, `settinglist.js`, `access.js`, `records.js`,
  `upload.js`, `report.js` and `selfupdate.js`. Read the one you
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
- **The catalog's own words are identifiers, not labels.** A badge once said
  "x86" for a 32-bit image because it drew the raw value (v0.5.0 fixed it).
  Anything drawn from a catalog field goes through a map to what a person
  calls it.
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

- An explicit *Cancel* in the name-clash dialog, which is currently "do
  nothing" plus a message saying nothing has changed. Add a real Cancel
  button that clears the row?
- Anything from the review doc listed as done that doesn't feel done when
  used on a real drive.

[#1]: https://github.com/ZachCurry13/isoshelf/issues/1
[#2]: https://github.com/ZachCurry13/isoshelf/issues/2
[#3]: https://github.com/ZachCurry13/isoshelf/issues/3
[#4]: https://github.com/ZachCurry13/isoshelf/issues/4
[#5]: https://github.com/ZachCurry13/isoshelf/issues/5
[#6]: https://github.com/ZachCurry13/isoshelf/issues/6
[#7]: https://github.com/ZachCurry13/isoshelf/issues/7
[#11]: https://github.com/ZachCurry13/isoshelf/issues/11
[#12]: https://github.com/ZachCurry13/isoshelf/issues/12
[#55]: https://github.com/ZachCurry13/isoshelf/issues/55
[#62]: https://github.com/ZachCurry13/isoshelf/issues/62
