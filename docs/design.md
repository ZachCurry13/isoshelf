# isoshelf: design and rules

The full design: hard rules, architecture, catalog format, milestones and the
web UI. This was CLAUDE.md until 2026-09-18; CLAUDE.md is now a short summary
that points here. Where things stand and what comes next: [STATUS.md](STATUS.md).

Working name; rename freely. An app (Windows and Linux; on a Mac, the container) that
inventories, update-checks, downloads and verifies the bootable images on a
Ventoy drive or in any image folder (NAS share, Proxmox ISO storage). It can be
installed on a PC, run portably from the drive itself, or (later) run as a
server in Docker/LXC. The UI is a web page served by the app. Independent
project, not affiliated with Ventoy.

## Hard rules

- Never touch partitions, bootloaders, or Ventoy's `/ventoy` folder. Only work
  with image files inside the folder the user picks.
- Never delete or overwrite anything the user hasn't chosen to replace. One
  answer in Settings says what every update does with the old file - replace,
  archive, or keep both (`old_files`; replace until it is changed, v0.3.1) -
  and a **pinned** file (`FileRecord.Pinned`, v0.7.0) is never replaced,
  archived or tidied away by an update: the new file downloads beside it.
  From v0.3.0 to v0.7.0 each image carried its own answer, set in its details
  panel; opening a folder now turns those into pins (keep both) or drops them
  (anything else, which then follows Settings), once, and the page says what
  it did (`MigrateChoices`). Replace = download -> verify -> rename into place
  -> only then delete the old file(s) of that same track.
- A download without a published checksum ("unverified") never replaces
  anything on its own: the old file stays until the user confirms that item.
- Only ever delete image files inside the folder the user picked: recognized
  images, and unrecognized ones such as a renamed Windows ISO (decided
  2026-09-17, replacing "only files matched to a catalog entry"). Never other
  files (notes, archives, anything without an image extension or image
  content), and never anything outside that folder.
- Removing an image asks first, per file, and offers both: move aside into
  `<target>/.isoshelf/removed/` (instant and undoable; the space is freed when
  the user empties it, and isoshelf shows how much it holds), or delete now.
  An update is the one thing that doesn't stop to ask, because the image's
  own choice above already is the answer - given once, in plain sight in its
  panel, and changeable at any time (decided 2026-09-18, built in v0.3.0;
  this is the wording that decision said to come back and fix). Either way
  nothing goes without the user having chosen it, and archiving is always
  offered instead of deleting.
- A mismatch against a published checksum always blocks placement. No published
  checksum -> allow, but mark the file "unverified".
- Updates never change an entry's architecture, edition or channel. A 32-bit
  track never "updates" to a 64-bit image.
- Checksum files and signatures come only from the official HTTPS origin. Image
  bytes may come from mirrors because they are verified against those.
- Portable mode writes nothing to the host computer: config, catalog, state,
  history, temp files and logs all stay on the drive.
- "Make bootable" fix-ups are manual, per file, and confirmed. Keep the
  original until the result is complete and checked.
- isoshelf never hosts, mirrors or re-serves anyone's image. It downloads from
  the project's own servers to the user's own machine, and that stays true in
  server mode: a server downloads for itself, never for the public.
- Never automate a vendor's download flow that isn't meant to be automated,
  and never touch licensing: Windows entries are `manual` (a link to
  Microsoft's page), with no product keys and nothing about activation.
- Nothing is ever circumvented: no CAPTCHA solving, no rate-limit dodging, no
  paywalls or DRM. Requests identify themselves as isoshelf with its version.
- Project names and logos are only ever used to say which image a file is,
  never as isoshelf's own branding, and always with the "not affiliated"
  notice. If a project asks to be delisted, remove its entry.

## Stack

- Go. One binary per OS, no runtime dependencies. Everything builds with
  `CGO_ENABLED=0`.
- UI: a web UI served by the same binary (`internal/web`), with HTML/CSS/JS
  embedded via `embed` and no Node build step. Only `internal/web` knows about
  HTTP handlers and HTML.
  - Desktop and portable: listen on `127.0.0.1` and open the browser. Guard with
    a random per-launch token and check `Host`/`Origin` headers (blocks DNS
    rebinding and cross-site requests).
  - Server (`isoshelf ui --listen <address>`, which the container does):
    reached like other homelab apps at `http://<nas-ip>:<port>`, with a
    username and password.
- No native macOS build (decided 2026-09-19): Mac users run the container
  with Docker Desktop. A native one would need a Mac or a macOS CI runner, and
  Apple's signing and notarization for a first launch without warnings.
- Dependencies today: `github.com/pelletier/go-toml/v2` and nothing else.
  Free space comes from the standard library's `syscall` (`statfs`, and
  `GetDiskFreeSpaceExW` on Windows).
- Accepted for when they are built, not in `go.mod` yet: OpenPGP
  ([#5](https://github.com/ZachCurry13/isoshelf/issues/5)) with
  `github.com/ProtonMail/go-crypto` (not the deprecated `x/crypto/openpgp`);
  extracting archives for Make bootable
  ([#3](https://github.com/ZachCurry13/isoshelf/issues/3)), pure Go only:
  stdlib `compress/gzip` and `archive/zip`, `github.com/ulikunitz/xz`,
  `github.com/bodgit/sevenzip`.
- Catalog: TOML, regexes in single-quoted literal strings. Parsed with
  `github.com/pelletier/go-toml/v2` in strict mode, so a misspelled key is an
  error with a line number.

## Architecture

Every catalog entry runs through four stages:

1. **Source** (`internal/source`) - "what's the latest version?"
   - `endoflife`: endoflife.date API v1,
     `https://endoflife.date/api/v1/products/{product}/`. `channel` is
     `latest`, `lts`, or a pinned cycle. Flavors reuse the parent product
     (Kubuntu uses `ubuntu`). Cycles are ordered by release date; when a cycle
     has no `latest`, its name is the version. The release carries every
     track cycle, so a file's own cycle can be checked for EOL.
   - `github`: newest release whose tag matches a regex. File comes from the
     release assets (verify with the per-asset SHA-256 `digest` in the REST
     API), or the release is only a version label and the file lives elsewhere.
     Drafts and prereleases never count.
   - `listing`: a checksum file, a mirror's directory index, or a JSON endpoint;
     version extracted by regex; the highest version found wins.
   - `static`: version + URL pinned by hand in the catalog.
   - `manual`: inventory only; optional known-good SHA-256 list; "open download
     page" action.
2. **Resolver** (`internal/resolve`) - expands `{cycle}`/`{version}` templates,
   fetches the checksum manifest, finds the exact filename by regex (or, when
   there's no manifest or its name uses `{file}`, from the filename a listing
   source already matched, else the directory index at `base`). A manifest
   that redirects to another host is refused. Output:
   `Artifact{Filename, URLs, Size, Checksum}`.
3. **Verifier** (`internal/verify`) - GNU (`hash  file`) and BSD
   (`SHA256 (file) = hash`) manifests; MD5/SHA-1 count as weak integrity only.
   Signatures are planned ([#5](https://github.com/ZachCurry13/isoshelf/issues/5)),
   not built: detached over manifest, clearsigned manifest, detached over the
   image, with armored keys whose fingerprints are pinned in the catalog.
4. **Fetcher** (`internal/fetch`) - `.part` files in `<target>/.isoshelf/partial/`
   plus a sidecar (URL, ETag, offset, SHA-256 state), resume via `Range` +
   `If-Range`, backoff on timeouts, 5xx and HTTP/2 stream resets (seen on
   repo.almalinux.org), no retry on 404, GitHub rate-limit
   reset times, mirrors only when a checksum can prove the bytes. A checksum
   mismatch deletes the partial file and places nothing. `BeforePlace` runs
   after verification and just before the rename, which is how the old file is
   moved aside for images whose filename never changes. Downloads are queued
   by the web UI and run one at a time (see Web UI); still to do: 2 at once,
   1 per host.

5. **Update** (`internal/update`) - resolves the entry again, downloads,
   places, records it in state, then keeps, moves aside or deletes the old
   files of that track. A download with no published checksum never removes
   anything: old files are kept, and an old file with the same name is moved
   aside, never deleted, whatever the page asked for. It also removes files
   the user no longer wants, and empties `<target>/.isoshelf/removed/`.

`internal/inventory` runs one scan or check from start to finish (load state,
scan, hash, check online, save state and mirror) with progress callbacks; the
CLI and the web UI both call it. `check.Report.JSON()` is the shared JSON shape.

Supporting packages: `internal/remote` fetches small documents (HTTPS only,
including redirects; retries timeouts and 5xx but not 4xx; caches per client;
sends an optional GitHub token and reports rate-limit reset times).
`internal/version` compares versions naturally (`22.10` > `22.4`).

Core packages take a target folder and report progress through plain Go
callbacks or channels, with no UI assumptions, so the CLI, the local web UI and
server mode all share them.

Images whose filename never changes (`bazzite-...-stable-...iso`,
`Core-current.iso`, `HBCD_PE_x64.iso`): "update available" means the published
checksum differs from the checksum recorded in drive state. A new GitHub tag
alone is not an update.

## Targets, state and scanning

- Code: `internal/scan` walks a target, `internal/sniff` identifies content,
  `internal/state` keeps state, mirrors and hashes, `internal/sampledrive`
  holds the sample drive and the maintainer's Proxmox folder as test fixtures.
- A target is any folder the user picks: a Ventoy drive, a folder on a NAS
  share, or Proxmox ISO storage. Each target has a profile, saved in its state.
  The profile only changes which files count as bootable and how deep the scan
  goes:
  - `folder` (default since v0.2.1): recursive; every image counts, including
    compressed card images (`.img.xz`, `.img.gz`, `.zip`), and nothing is
    called "not bootable", because nothing boots from a NAS share directly.
  - `ventoy`: recursive; bootable = `.iso .wim .img .vhd .vhdx .efi`.
    Suggested automatically when the folder has a `ventoy` folder.
  - `proxmox`: top level only; bootable = `.iso .img`, case-insensitive
    (Proxmox's `$ISO_EXT_RE_0`; it lists nothing else and ignores subfolders).
    Suggested automatically when the path ends in `template/iso`.
- Downloads and fix-up output are staged in `<target>/.isoshelf/`, so the final
  rename never crosses filesystems (NAS shares, or a portable app on a USB drive
  managing a NAS folder).
- State lives in `.isoshelf/state.json` in the chosen folder: profile, placed
  files (entry, version, filename, SHA-256, source URL, date), the per-track
  keep/replace and star choices, manual assignments for renamed files, the
  archive of images that have left (`past`), and scan history
  (last 100 scans). A random `target_id` tells targets apart when drive letters
  change. File records stay valid while size and modification time are
  unchanged; a changed file loses its hash and assignment.
- **Where a folder's records live is the folder's own answer** (decision 10),
  kept in the settings file under `folder_records`, keyed by the folder's path.
  `state.Home` resolves it: the empty Home is the default and the one to
  prefer, since a drive that carries its own records arrives at another
  computer knowing itself. A Home that names a folder holds every folder's
  records together in `<home>/records/<hash of the folder's path>.json` - a
  path can't be a file name, and a hash is a fixed length whatever the path.
  The cost is that they are then found by that path, so the same drive at a
  different letter starts with nothing.
  - Changing the answer **moves** the records (`state.Move`). Anything else
    looks like isoshelf forgot the folder. Records already at the destination
    win and the old ones are left alone; nothing isoshelf wrote is deleted
    without the user choosing it.
  - It **asks first** (since v0.5.3). `POST /api/records/plan` runs
    `state.Plan`, which works out what `Move` would do and does none of it;
    `Move` is built on `Plan`, so the question and the move can't disagree.
    The page names both files and what stays behind, or - when records are
    already waiting where these would go - which set wins and when each was
    last saved. A folder nothing has been saved about yet has nothing to ask
    about. The answer to the move carries `records.moved` once, and the note
    under the setting says what happened. Taking the last file out of `.isoshelf`
    removes that folder if it is then empty, and never if the archive or a
    part-finished download is still in it.
  - Only the records move. The archive and staged downloads stay in the
    folder: archiving is a rename, and a rename across disks is a copy of
    every byte.
  - Nothing may be running while they move, or a scan that saved afterwards
    would save to the old place.
- History and the usual set are mirrored to `<config>/targets/<target_id>.json`
  so they survive a dead drive (not in portable mode; offer "Export usual set" there instead).
  Since v0.4.3 a mirror also holds how many files the folder had and how many
  bytes they took, which is what the **folders isoshelf remembers** list in
  the folder chooser shows, along with when each was last saved. That list is
  read only from these copies, never from the drives: a folder on it may be a
  NAS that is asleep or a stick in a drawer. **Forget** deletes that one
  mirror and the folder's records answer, and nothing on the drive at all.
- Usual set = starred entries + entries seen in at least 2 of the last 10
  scans, less any the user has stopped expecting. "Missing" means
  missing from the usual set, not from the whole catalog. Rebuild offers the
  usual set as a preset.
- A missing image offers one way back (v0.7.1, #55, `missing.js`):
  **Restore** when its file is still in the archive (instant, and the very
  file that was there), else **Download again** when isoshelf can download
  it, else **Download page**. Filtering to missing images offers **Download
  all**, through the same checklist as Update all. **Stop expecting it**, in
  the details panel, is for an image removed on purpose: it sets
  `Track.NotExpected`, which takes the image out of the usual set and off its
  star until a scan finds it in the folder again (`RecordScan` clears it).
  The server drops that row from the report at once rather than rebuilding
  the report, which would lose the last check (`dropMissingLocked`); the
  same goes for unstarring a missing image.
- Scanner lists files that match a catalog entry or have a bootable extension.
  It skips `ventoy/` (top level), `.isoshelf/`, `System Volume Information` and
  the portable app folder, and reports the size of trash folders (`.Trash-*`,
  `.Trashes`, `$RECYCLE.BIN`) without listing their contents.
- Flag files that match a catalog entry but have an extension the profile
  doesn't list (e.g. `.bin`) as "not bootable" and offer "Make bootable".
- First scan hashes fixed-filename images in the background (cancellable) and
  records the results.
- Several files for one entry are grouped, with a "keep newest" action.
- Statuses: up to date, update available, missing, manual, unverified, checksum
  mismatch, EOL (endoflife.date `isEol`), not bootable, unrecognized, check
  failed (showing the real error), plus unknown (can't decide, e.g. not hashed
  yet) and not checked (before the online check).
- Status rules (`internal/check`):
  - Fixed-name: compare the published SHA-256 with the recorded hash.
  - Versioned with an artifact: up to date if the resolved filename is the file
    on disk (checksum mismatch if a recorded hash differs); otherwise compare
    versions. Check-only: compare versions.
  - EOL comes from the file's own endoflife cycle (the pinned channel's cycle
    when pinned). "Up to date" + EOL shows as EOL; an update keeps its status
    with an EOL flag.
  - Not bootable keeps its status but still shows the latest version.
  - Files of one entry with versions are grouped; older ones get an "older
    copy" note. Files without a version in the name are never guessed.

## Portable mode

- Code: `internal/appdir`. Portable: config in the app folder, temp in its
  `tmp/`, default target = the folder holding the app folder. Installed:
  `os.UserConfigDir()/isoshelf`.
- Releases include a portable folder (`isoshelf/` with
  `isoshelf-windows-amd64.exe`, `isoshelf-linux-amd64`, `isoshelf-linux-arm64`
  and a `portable` marker file) that the user copies onto the drive.
- Marker next to the executable -> portable mode: store everything in that
  folder, keep temp files on the drive (downloads are staged in the target, see
  above), and default the target to the drive the app is running from.
- No autorun. Windows ignores program autorun from USB drives (since Windows 7)
  and Linux/macOS don't auto-launch either; the user starts the app. Optional,
  off by default: an `autorun.inf` with icon/label only. Never overwrite an
  existing one, and warn that some antivirus tools flag `autorun.inf`.
- README must cover Linux mounts that don't allow running programs from the
  drive (copy the binary to the home folder and run it from there).

## Make bootable (fix-ups)

- Identify files by content (magic bytes), never by extension alone: ISO9660,
  raw CD sectors (bin/cue), MBR/GPT disk image, xz/gzip/zip/7z archive.
- Known fix-ups:
  - Rename: ChromeOS-family `.bin` (FydeOS, CloudReady) -> `.img`, as Ventoy's
    FydeOS/CloudReady docs require.
  - Extract: `.img.xz`, `.img.gz`, `.zip`, `.7z` -> the image inside.
  - Convert: raw CD `.bin`/`.cue` data track -> `.iso`.
- Anything else: manual "Rename to..." with a warning that Ventoy may still not
  boot it.
- Catalog entries may declare a fix-up (`fixup = "rename:.img"`,
  `fixup = "extract"`) so downloads get it applied automatically.
- Extract and convert need a free-space preflight; rename is instant.

## Catalog

- Default catalog embedded in the binary; user copy in the OS config dir, or in
  the app folder in portable mode.
- Code: `internal/catalog`. The file starts with `schema = 1` and a
  `revision`, written as the date it last changed (`20260918`). Raise the
  revision with every catalog change: it is how a downloaded catalog and the
  one built into the binary are told apart, so installing a newer isoshelf
  never steps back to a list downloaded months ago (`Catalog.Newer`).
- One `[[entry]]` = one track:
  - `id` (lowercase, digits, dashes), `name`, `arch` (`x86_64`, `x86`,
    `arm64`, `arm` (32-bit), `multi`).
  - `match`: regex over the whole filename. Needs a `version` group, unless
    `fixed_name = true` (then it must not have one) or the source is manual
    (then it's optional).
  - `samples`: at least one real filename; each must match its own entry only.
  - Optional: `fixed_name`, `page`, `fixup` (`extract`, `convert`,
    `rename:<ext>`), `known_hashes` (SHA-256), `size` (roughly how big the
    download is, in bytes; measured by `record -sizes`, refused if too small
    or too large to be an image).
  - For the UI: `category` (desktop, gaming, server, boards, security,
    rescue, windows, other — "boards" is the Raspberry Pi and other
    single-board computers, "gaming" includes handhelds), `family` (groups one
    product's tracks), `site`, `forum`, `popular` (a hand-picked hint from
    public round-ups, not a rating), and `icon` + `icon_color` (a Simple Icons
    name and brand color).
  - `caution`: one line of fact worth knowing before using the image
    (support ended on a date, an unofficial modification, a preview that
    expires), at most 200 characters. The page shows a ⚠ beside the name,
    with the same mark for anything the online check finds is end of life,
    and a key under the list when any row has one. It is never a block:
    people keep old images on purpose. Keep cautions to verifiable facts about
    support and provenance, never opinions about a project.
- `[entry.source]`: `type` plus only that type's fields. endoflife: `product`,
  `channel`, optional `cycles` (regex over cycle names; only matching cycles
  belong to the track, e.g. to keep LMDE out of Linux Mint). github: `repo`,
  `tag` (regex), optional `asset` (regex). listing: `url` (https), `regex`
  (with a `version` group). static: `version`. manual: none.
- `[entry.artifact]` says where to download. Not allowed for manual or github
  with `asset` (the file and its digest come from the release). Optional
  otherwise: without it the entry is check-only (version compare and EOL, plus
  "open download page"). Fields: `base` (https, ends in `/`), `file` (regex),
  optional `manifest`, `signature` (`manifest`, `clearsigned` or `image`),
  `sig`, `key`, `mirrors` (need a `manifest`).
- `page` is required when there's nothing to download: manual entries and
  check-only entries.
- Placeholders: `{version}` everywhere, `{cycle}` for endoflife, `{tag}` for
  github, `{file}` (the resolved filename) only in `manifest` and `sig`. Values
  are regex-escaped inside `file`.
- A fixed-name entry that isn't manual needs a published checksum
  (`artifact.manifest` or a GitHub `asset`), since that is how updates show up.
- `[keys.<name>]`: `file` (armored key, relative to the catalog file) and
  `fingerprints`.
- Validation: regexes compile, templates are valid, referenced keys exist, each
  sample filename matches exactly one entry. Every problem is reported at once.
- Scope: the sample drive is only an example. The default catalog should cover
  common and trending images (desktop, server/homelab, rescue tools), so users
  can add images that aren't on their target yet. It has 86 entries: the
  sample drive, the maintainer's Proxmox folder, the images people ask for
  first, and what people are talking about. Anything worth knowing goes in,
  even when isoshelf can't download it: an entry with only a page link still
  names the file and sends people to the right place. Add more in batches,
  checking each one live before it goes in.
  Why each entry is set up the way it is, and the wish list:
  `docs/catalog-sources.md`.
- A few entries pin a release number in their URLs because nothing else gives
  the version their checksum file is named after (Fedora). Those need raising
  by hand each release; `docs/catalog-sources.md` lists them.
- Every catalog URL is the final address: tests can't replay redirects, and a
  checksum file that redirects to another host is refused.
- Maintenance (since 2026-09-18): `.github/workflows/catalog-check.yml` checks
  every entry live each Monday and on every pull request that changes the
  catalog; the weekly problems go into one `catalog` issue. A scheduled
  Claude job (a cloud routine) then works through the `catalog` issues and
  the wish list and opens one pull request for the maintainer to review.
- Later: one file per distro under `catalog/`, validated in CI.

## Milestones

**v0.1 - read-only** (the only writes are to `.isoshelf/`)
1. Catalog loader + validation. *Done.*
2. Scanner + filename matching + content sniffing + target profiles, with table
   tests built from the sample drive. *Done.*
3. Drive state + usual-set history, including portable-mode storage. *Done.*
4. Sources: `endoflife`, `github`, `listing`, `manual` (check only). *Done.*
5. CLI: `isoshelf scan <folder>` and `isoshelf check <folder>` print a status
   table; `--json` for machine output. Includes the app update notice (see
   "App updates"). *Done.*
6. Local web UI (opened in the browser) showing the same table, plus catalog
   entries not on the target (listed only; adding them needs v0.2 downloads).
   *Done.*

**v0.2** - downloads and the actions around them. *Released as v0.2.0 on
2026-09-18, the first released version.*
1. Downloads: resumable fetch, checksum verification, place into the target.
   An Update button per image plus "Update all". *Done.*
2. Delete: remove images the user no longer wants. *Done.*
3. The archive of images that have left, with Restore and Download again,
   plus filters, sorting, logos and links. *Done.*
4. Assign: suggest what an unrecognized file is and confirm it (below).
   *Done.*
5. Adding catalog images that aren't on the target (the "Add" button), and
   a download queue for Add and Update. *Done.*
6. Growing the catalog without a new release (below). *Done.*
Moved later, and still to come: installing older versions with a hold
(below), Make bootable fix-ups, a CLI `isoshelf update` command (the web UI
has it), two downloads at once from different hosts, and OpenPGP signature
checking.

**v0.3** - a redesign of the page and Settings (decided 2026-09-18; the list
of decisions is in [STATUS.md](STATUS.md)).
**v0.4** - running on a NAS in a container, with a login; where each folder's
records live; updating images by itself; copying from another isoshelf.
**v0.5** - the page redone, and a round of polish and correctness.
**v0.6** - isoshelf updating itself (below).
**Next** - the order in [TODO.md](TODO.md): a simpler page (v0.7.0), the
items moved above, rebuild and repair modes, and an official TrueNAS app for
1.0.
**Releases** - GitHub Actions matrix (Windows + Linux) on `v*` tags; attach
binaries, the portable zip, and `SHA256SUMS` to the release. Every file
attached to a release carries the version
(`isoshelf-vX.Y.Z-windows-amd64.exe`), because that is what people download
and keep. The binaries *inside* the portable zip keep the plain name
(`isoshelf-windows-amd64.exe`): that is the file run from the drive, and the
one self-update replaces in place, so it must not change every release. The
updater finds its asset by pattern (`isoshelf-<tag>-` at the front,
`-windows-amd64.exe` at the end) rather than by an exact name, which would go
stale every release. Releases up to v0.3.0 attached plain names; those
releases keep them, so anything reading old releases has to cope with both
shapes. The update check itself reads only `tag_name` and `html_url`, so it
is unaffected either way.
- **isoshelf updates itself** (decisions 13 and 14, built in v0.6.0;
  `internal/appupdate`, `internal/web/selfupdate.go`,
  `cmd/isoshelf/handover.go`, `static/selfupdate.js`). The shape, and why:
  - **Signed.** The release workflow signs `SHA256SUMS` with an Ed25519 key
    (`internal/appupdate/sign`, key from the `RELEASE_SIGNING_KEY` secret);
    the public half is `internal/appupdate/release.pub`, embedded in every
    build. `Prepare` refuses a release without `SHA256SUMS.sig`, with a
    signature that doesn't verify, or whose program doesn't match its line.
    A build with no key can't update itself at all. Standard library only.
    The key is made once by `internal/appupdate/keygen`, which refuses to
    replace an existing key without `-replace`: a new key strands every copy
    already installed.
  - **Only on request.** Nothing downloads until **Update now** (the
    maintainer's choice, 2026-09-23). It waits for image downloads, scans and
    uploads to finish, and nothing new starts once it is restarting.
  - **What it replaces.** In portable mode every program in the folder, so a
    stick never carries two versions; otherwise the running program. A
    single download named for its version takes the plain name. Everything
    is staged in `.isoshelf-update` beside the program, so putting it in
    place is a rename.
  - **Nothing is deleted until the new one runs.** `Swap` renames each old
    program to `.old` and the new one into place, and writes `swapped.json`.
    The old program stops listening and starts the new one with a handover
    (`ISOSHELF_HANDOVER`: the port, the staging folder, the old version) and
    the link's token (`ISOSHELF_TOKEN`), so the page's address and cookie
    still work. On Linux that is `exec` - the same process, which a terminal
    and a service manager both keep - and on Windows a new process in the
    same console. The new program tries for its port for 15 seconds, and
    once serving deletes the backups (`Confirm`). If it can't listen, or
    panics first, it undoes the swap and starts the old program, which tells
    the page why.
  - **Windows can rename a running program but not delete it**, so undoing
    moves the failed program into the staging folder rather than deleting it,
    and `Tidy` clears such leftovers at the next normal start.
    `cmd/isoshelf/handover_test.go` builds the real program and runs both
    paths, and the undo test was checked by breaking it.
  - **Not in a container** (`ISOSHELF_CONTAINER`, set in the Dockerfile): a
    container is updated by pulling the image.

### The archive of images that have left

- `state.Past` remembers every image that leaves a folder: what it was, its
  version, size, hash and where it came from, when it was last seen, and how it
  went (`removed`, `moved-aside`, `replaced`, `vanished`). Capped at 500, newest
  first, and a path is forgotten as soon as that file is back.
- Files moved aside wait in `<target>/.isoshelf/removed/` and can be restored
  while they are there; catalog images can always be downloaded again.

### Assign: suggesting what an unrecognized file is

For files the catalog doesn't match by name, such as `Windows.iso` from the
Media Creation Tool. `internal/identify` suggests, the user confirms; nothing
is ever assigned silently, and no file is renamed or moved.

- `internal/sniff` reads the ISO 9660 primary volume descriptor while it works
  out the file's kind, from the same 64 KB it already reads: label, system,
  publisher, preparer, application and build time. Real labels are worth a
  lot (`Ubuntu 22.04.3 LTS amd64`, `CentOS 7 x86_64`, `Parrot home 6.2`) and
  some say nothing at all (`ISOIMAGE`, `ESD_ISO`, `PVE`).
- Evidence, strongest first: a checksum in `known_hashes`; the same file in the
  folder under another name (equal hashes, or the same size *and* the same
  label and build time); a file of the same checksum in the archive; then
  resemblance between the entry's name and the filename and label.
- Resemblance weighs rare words higher than common ones (a catalog-wide
  inverse frequency), so "qubes" counts for far more than "linux". A
  disagreeing architecture costs 40 points, and an entry whose sample
  filenames use another extension costs 25, which is what keeps an `.iso`
  from being offered as Ubuntu Core (`.img.xz`).
- Scores are shown in words: above 80 ("Very likely", "Almost certain") rests
  on evidence, below that on resemblance. Every guess carries its reason in
  plain language, and the whole catalog is one search box away.
- A confirmed answer is `state.Assign`: entry, version and `assigned: true`
  against that path, which `RecordScan` keeps until the file itself changes.
  "Forget this" is `state.Unassign`.
- The web API is `GET /api/guesses?path=` and `POST /api/identify`. Both work
  off the scan the server already has, so they touch no disk. Identifying
  rebuilds the offline report at once; the page starts the online check again
  if one had already run.

### Growing the catalog without a new release

The catalog is data, so it must be able to grow without shipping a binary. It
must also stay safe: a download address that nobody checked is exactly how the
wrong image gets trusted. So the split is deliberate.

- **The catalog updates itself.** *Done:* `internal/catupdate`. One address
  only (`SourceURL`, the catalog file in this repository), fetched over HTTPS
  at most once a day, written to a temporary file, parsed and validated, and
  only then renamed to `<config>/catalog-published.toml`. Anything wrong keeps
  the copy already there and says so in plain words. The result names what was
  added and removed ("Catalog updated: Fedora KDE and 2 more added.").
  - Load order (`loadCatalog` in `cmd/isoshelf`): `--catalog` flag, then the
    user's `catalog.toml`, then the downloaded copy, then the built-in one.
    The first two are "yours" and are never replaced or overwritten.
  - The server keeps the catalog behind its mutex (`s.cat`, read through
    `s.catalog()`), because a refresh can swap it while isoshelf runs. A swap
    rebuilds the offline report; new images turn up on the next scan.
  - The checkbox lives in `ui.json` as `catalog_auto` (absent = on). A test
    asserts the catalog in the repository would be accepted as an update, so a
    broken catalog can't be published.
- **Add my own image.** *Done:* `internal/usercat`. Anything unrecognized can
  be given a name, kind, architecture and (optionally) a page in the "What is
  this file?" dialog. isoshelf appends a `manual` entry to
  `<config>/catalog-mine.toml`, a file of the user's own that is merged onto
  whichever catalog is in use (`catalog.Merge`), so the published catalog
  underneath keeps updating itself. Entry ids start with `my-`. The file is
  only kept if the merged result still validates, so a name that clashes
  leaves nothing behind. Inventory only: no download address is ever invented.
- **Tell isoshelf about this image.** *Done:* the same dialog offers a link to
  a prefilled issue (filename, size, content kind, disc label). It opens in
  the browser for the user to read, change and send; isoshelf never sends
  anything itself, and nothing personal goes in it. That is how new download
  sources arrive: a person checks each one once.

### Default credentials

Some live and rescue images ship with a documented login - MX Linux's
`demo`/`demo`, and others like it. An image you boot once a year is exactly
the one whose password you will not remember, so it is worth carrying.

The rule is the checksum rule, for the same reason: **the credentials come
from the project's own documentation and nowhere else.** Not a forum, not
somebody else's wiki, not from memory. A project that doesn't document them
gets nothing written down. A password isoshelf guessed at is worse than no
password, because somebody will type it into a machine they care about.

They rot - defaults change between releases - so the entry keeps the address
it was read from and the page shows it. That is the "don't write down
anything that has to be maintained" rule bending as far as it will go: the
fact earns its place, so it has to arrive with the means to re-check it.

The page says what it is: *the project documents these as the live session's
defaults*, never anything implying isoshelf discovered them. Not yet built;
see TODO item 19 for the shape.

### Ventoy and other tools

Naming Ventoy to say isoshelf works with it is fine, and worth doing: many
people arrive through Ventoy. Keep it to plain text and a link to ventoy.net,
never their logo, and keep "independent project, not affiliated with Ventoy"
next to it.

### Older versions

For when a new release breaks something and the user needs the previous one.

- Sources can list every version still published, newest first: each
  endoflife cycle that belongs to the track (its latest version), each matching
  GitHub release, each version a listing shows. Manual and check-only entries
  can't. Only versions of the same track are offered.
- The chosen version is resolved and verified like any download. If the project
  no longer publishes it or its checksum, say so; never take checksums from
  anywhere but the official site.
- Installing an older version puts the track on hold (`hold` in state): checks
  show "held at <version>" instead of "update available", and the scheduler
  never updates or replaces a held track until the user releases the hold.
  The keep/replace checkbox applies as usual.

### App updates

- Release builds embed their version (`-ldflags "-X main.version=vX.Y.Z"`).
  Development builds report `dev` and never check.
- On start (at most once an hour, the same everywhere - there is no separate
  server-mode interval), ask the GitHub API for this repository's releases and
  take the newest worth offering. **The list, not `/releases/latest`**: that
  leaves pre-releases out, and a `-rc` build must not hide behind it for
  somebody already running one. A pre-release is only offered to somebody on
  a pre-release. If the answer is newer, show a notice with a link to the
  release notes: one line on stderr in the CLI, a link in the top bar of the
  page and again under Help in Settings. The check is turned off in Settings
  ("Tell me about new isoshelf versions"), or with `--no-update-check` or
  `ISOSHELF_NO_UPDATE_CHECK`; the last check time lives in the config folder,
  in `update-check.json`.
- Installing it is **Update now**, which checks the release's signed
  `SHA256SUMS` first; how, and why, is under *isoshelf updates itself* in
  Milestones. A container is updated by pulling the image.
- The repo must be public by the first release, or both the check and
  downloads fail for users.

### Server mode

Run isoshelf on the machine the images already live on and manage it from a
browser. Shipped in v0.4.0; see `docs/docker.md`.

- **Done:** `isoshelf ui --listen ADDRESS` - the same binary and page as the
  desktop, answering to the machine's own address instead of only localhost.
  A Docker image (static binary plus CA certificates), built for amd64 and
  arm64 and published to ghcr.io, which is also what a TrueNAS app is.
  `/healthz` and `/login` are the two paths outside the guard. The rest of
  the page - browsing the catalog, updating, the answer about old files,
  history - was already there and needed nothing.
- **Getting in** (v0.4.5, the maintainer's decision over keeping the token
  alone): a username and password, chosen on first run or given in
  `ISOSHELF_USERNAME`/`ISOSHELF_PASSWORD`. One login, not user accounts -
  isoshelf looks after a folder, and there is one person's worth of access to
  give. `internal/auth` keeps it: PBKDF2-HMAC-SHA256 with a random salt, from
  the standard library, so the one dependency stays one.
  - The session is **signed, not remembered**: the cookie says who and until
    when, with an HMAC under a key in the config folder. A restart therefore
    keeps everyone logged in, which matters because a NAS app restarts on
    every update, and isoshelf keeps no list of who is logged in. Signing out
    everywhere is throwing that key away.
  - **The link's secret is replaced, not joined** (v0.4.7, the maintainer's
    decision: "I want to get rid of tokens moving forward and only have a
    login screen"). It works until a password is set, so a new install can be
    opened at all, and stops the moment one exists - including a cookie
    somebody already holds, with no restart needed. Two ways in is two ways
    in, and the weaker one sat in a log.
  - The way back from a forgotten password is therefore not the page:
    `ISOSHELF_USERNAME`/`ISOSHELF_PASSWORD` at the next start, or
    `isoshelf password` from a shell on the machine. Both need the machine,
    which is the right bar, and neither is a second door on the network.
    `isoshelf password` does not hide what is typed: that needs either a
    second dependency or platform code for both systems, to guard against
    somebody standing behind you at your own NAS. It says so, and points at
    the environment variables.
  - The login form **cannot use the Origin check** every other change uses.
    isoshelf sends `Referrer-Policy: no-referrer`, and browsers then send
    `Origin: null` on a plain form post. So the form carries a random value
    set in a `SameSite=Strict` cookie and echoed in a hidden field; a form on
    another site can produce neither.
  - The page's inline stylesheet needs its **own hash in the content policy**,
    derived from the same constant so it cannot drift. Without it the login
    page still works and arrives unstyled.
- **Updating by itself** (v0.4.6, the maintainer's decision: fully automatic,
  end to end, over downloading-but-not-placing or checking only). On a
  schedule - daily or weekly - a check runs, and what it finds goes into the
  same download queue the Update button uses, with the same verification and
  the same answer about the old copy. `internal/web/autoupdate.go`.
  - **In a container it asks once** (v0.7.0): until somebody answers, the
    page asks whether the images should update by themselves, every day,
    every week, or not. It is off until then, like everywhere else. Yes
    starts the first run at once, so the question says what that run will
    do - how many updates, about how much to download, what happens to each
    old file - before anyone answers.
  - **Off unless turned on, and always will be.** It is the one thing
    isoshelf does that changes a drive while its owner isn't looking.
  - `removalFor` works out the answer, and has to agree exactly with
    `choiceFor` in `details.js`: what happens unattended must be what the
    page has been showing all along. A pinned file is kept by `update.Run`
    whatever either says.
  - It leaves room to spare (`roomToSpare`) rather than filling the folder,
    skips a folder that is already busy, and records when it last ran in the
    settings file - so a restart, which on a NAS is every app update, doesn't
    start another run straight away.
  - The settings file now has two writers, so every change to it goes through
    one lock (`updateSettings`). Without that, a switch and the scheduler
    noting the time can land on each other and one change vanishes.
- **Copying between isoshelfs** (v0.4.9, the maintainer's feature request).
  One isoshelf offers the images in its folder (`internal/web/share.go`,
  off by default); another asks it before the internet
  (`internal/peer`, `internal/web/peer.go`).
  - **The peer is reached, never trusted.** The checksum comes from the
    project's own HTTPS site as always, and `update.Options.Nearer` is only
    asked when there is one - without a checksum there is nothing to check a
    stranger's bytes against, so the project's own site is the only place
    isoshelf will look. `fetch` falls through to the next place on a
    mismatch, so a wrong copy costs a fall back rather than the download.
  - **Asked for by hash, not by name.** A name alone would say yes to a stale
    copy, which for a fixed-name image is exactly the file being replaced.
    So sharing needs hashes: with it on, scans hash every recognized image
    (`NeedsHash(.., all)`) rather than only the fixed-name ones. Images
    isoshelf downloaded already carry theirs from the download.
  - **The name is never joined onto the folder.** `sharedFile` looks the name
    up in the records and returns the path isoshelf wrote there, so a name
    that isn't a record isn't a path - and the size and modification time
    have to match the record, or the file has changed since anybody hashed it.
  - **The sharer records which drives it served** (`internal/web/served.go`),
    keyed by the asking folder's target id. That list is most of what #11
    needs to rebuild a lost drive.
  - **Copying anything the server has** (v0.8.1, #62 part two). The sharing
    isoshelf lists every hashed file with its entry, version and `Origin`
    (`/api/share/list`); the laptop asks through `/api/peer/files`, leaves
    out what its folder has by hash or name, and marks catalog images "On
    your server" in Add images, with the rest folded under "Also on your
    server" (`fromserver.js`). **Copy** (`/api/peer/copy`) queues a job that
    downloads from the server checked against the server's hash - proof
    that it arrived as it is there, nothing more - so it is recorded as a
    copy with the server's account in `Before` and never as checked; the
    next check compares it with the published checksum like any file. A
    copy never lands on a name that's taken. What the server was told by
    hand about a file (an assignment) comes across as an assignment.
  - **Not done:** finding the other isoshelf by itself. Doing that properly
    means mDNS, which means a second dependency or a fair amount of protocol
    code, and is a decision rather than an omission. Until then the address
    is typed in.
- **Where each file came from** (v0.8.0, #62 part one; the maintainer asked
  for it wanting to be sure an image had come from the right source at
  least once). `state.Origin`, on every `FileRecord`: how the file arrived
  (`download`, `copy` from another isoshelf, `upload` from the page, or
  empty for a file found in the folder), from where and when, and - only
  when its hash matched a published checksum - that checksum's address and
  when (`Checked`, `CheckedAt`). `Before` holds what the isoshelf a copy
  came from said about its own copy.
  - Filled in by `update.Run` as it places a download (`originOf`: a copy
    from the server that matched the project's checksum is as proven as a
    download, because the checksum never came from the copy), by an upload,
    and by any check that finds a file's hash equal to the published one
    (`check.Item.Matched`, recorded by `inventory.Run` through
    `MarkChecked`). It belongs to the bytes and goes when the file changes.
  - **Check it** (`/api/checkfile`) is an ordinary run that hashes one more
    file (`inventory.Options.HashPaths`) and asks the project if the answer
    isoshelf has is stale (`askToProve`, even with checking by itself off).
    Only on request: the maintainer's answer to "hash by itself?" was no.
  - The maintainer asked for a record that would "at least make it look
    like" a file had been checked. Declined: a false record is worse than
    none, and a real check is available for every catalog image.
- **Not done:** a Proxmox LXC with the ISO storage bind-mounted, documented.
  An official TrueNAS store app, which is the 1.0 goal - `deploy/truenas/`
  has the start of one.

### CLI (`cmd/isoshelf`)

- `scan [flags] [folder]` (offline) and `check [flags] [folder]` (online);
  flags may come before or after the folder: `--profile folder|ventoy|proxmox`
  (saved in state), `--json`, `--catalog FILE`, `--no-hash`, `--no-update-check`.
- The folder defaults to the drive in portable mode and is required otherwise.
- Catalog: `--catalog`, else `<config>/catalog.toml` if present, else built in.
- `password [username]` sets the page's username and password from a shell
  on the machine; `version` prints the version; `help` lists everything.
- Env: `GITHUB_TOKEN` (rate limit), `ISOSHELF_NO_UPDATE_CHECK`,
  `ISOSHELF_USERNAME`/`ISOSHELF_PASSWORD` (the login, see Server mode).
- Both commands record the scan, hash fixed-name images (progress only when
  stderr is a terminal), save state and (not portable) the mirror. A state
  that can't be saved (read-only share) is a warning, not an error.
- Exit codes: 0 done, 1 failed, 2 usage error. The JSON field names are an
  interface for scripts; change them with care.
- Live smoke test: build it and run `check` on a folder of stand-in files named
  like the sample drive. This is how the kernel.org redirect for Linux Mint was
  found (the catalog now uses `mirrors.edge.kernel.org`).

### Web UI (`internal/web`)

- `isoshelf` with no arguments (double-click) or `isoshelf ui [--port N]
  [--no-browser] [--catalog FILE] [folder]` listens on 127.0.0.1 (any free port
  by default), prints a link with a random token and opens the browser.
- Security: the token link sets an HttpOnly, SameSite=Strict cookie; every
  request needs it and a localhost Host header (DNS rebinding); changes also
  need the `X-Isoshelf: 1` header and a same-origin Origin; a self-only CSP. The
  page inserts text from the drive with textContent only, never as HTML.
- API: `GET /api/state`, `/api/catalog`, `/api/browse?path=`; `POST /api/target`
  (folder + profile), `/api/scan`, `/api/check`, `/api/cancel`, `/api/track`
  (keep_old, starred), `/api/update` (queues a download), `/api/queue/move`,
  `/api/queue/drop`, `/api/queue/clear`. A scan and a download have a slot
  each (`s.scanning`, `s.downloading`) and run side by side in the background,
  one of each at a time; the page polls `/api/state` while either runs, draws
  the scan from `run` and the download from `downloads.current`, which carries
  its own progress. A scan waits only for another scan (`scanBusyLocked`);
  switching folders and emptying the archive wait for both (`busyLocked`);
  removing, archiving, identifying, putting back and stars wait only for a
  scan (`scanningLocked`). Nobody saves their whole copy of the folder's state
  over the file: each writer keeps the copy from before its change and
  `state.Merge` carries only that change onto the file as it is on disk
  (`state.SaveOnto`, statefile.go). A scan saves that way too, so a download
  that places a file while it reads the folder keeps that file's record.
- The folder picker lists subfolders through `/api/browse` (browsers can't see
  the computer's folders), with drives or mount points and recent folders
  (from the mirrors; none in portable mode). The last folder is remembered in
  `<config>/ui.json`.
- A filled star means the user starred the image: it is a favorite, sorts
  first, and is reported if it goes missing. The replace switch shows only for
  entries that can download.
- Menus (`details.menu`) are pinned to the window by `placeMenu` and wired by
  `wireMenu`: only one open at a time, opening upwards when there is more room
  there, and scrolling when a long list of filters fits neither way. Every
  menu needs the wiring - the Filter menu went without it until v0.3.3, so on
  a full drive it hung off the bottom of the window.
- A download that would land on top of a file already in the folder stops and
  asks (v0.3.3). `update.ErrSameName` is the one failure that is really a
  question: the server marks that job `conflict`, and the page offers
  "Archive the old one" and "Replace it" instead of a Try again that would
  fail the same way. Nothing in the folder has changed when it asks.
- The words are American English (the maintainer's decision, v0.3.3):
  *Favorites*, *color*. "Restore" is the word for bringing a file back from
  the archive, everywhere.
- One **Refresh** button (v0.3.2) replaced *Scan* and *Check for updates*:
  read the folder again and ask every project again. Scanning without going
  online is what turning the setting off does, not a second button.
- The page (v0.3.0) is one page with a sticky jump bar: Your images, Add
  images, Archive, History. Above the list, one line says what wants doing
  (`renderTodo` in `summary.js`, v0.7.0; it was a card for each until then):
  `41 updates · 13 older · 3 unrecognized · Archive 20.9 GB [Update 31]`.
  Each count shows exactly those images; Update all is the one button. The list is a favorite star and four columns - image (file,
  size and date underneath), version ("22.04 -> 24.04"), status, actions - and clicking a
  row opens the details panel: everything about one image, its links, and
  the pin below. Statuses are shown in plain words (`STATUS_WORDS`), each
  explaining itself; the report keeps its own words for the CLI and JSON.
- What happens to old files is one answer in Settings, followed by every
  update, so updating never stops to ask; a file pinned in its details panel
  is the exception, kept whatever comes, and left out of the older versions
  (`isOlder`). "Update all" and "clear older versions" share one checklist
  dialog (`pickFiles`, `checklist.js`); filtering to older versions offers
  Review and clear.
- The list filters by kind, architecture, updates, favorites, older
  versions and the caution mark through one Filter menu; everything switched
  on shows as a chip above the list. Ticking filters at once; the menu's
  footer has Clear all and Done, and its list scrolls above them. Clicking
  a count above the list clears every other filter first, so it shows
  exactly what was counted. Sorting is a menu and the three column
  headings (empty cells last).
  The choices live in the browser's localStorage. When a filter hides
  everything, the empty message names the filters and offers to clear them.
- "More in the catalog" has its own copy of those filters, plus how an image
  updates, "fits in this folder" and "popular", and sorts by name, largest,
  smallest, popular or kind. The two sets are deliberately separate: one list answers what have
  I got, the other what could I add. Each entry shows about how big its
  download is, and one too big for the room left says so instead of failing
  part way through.
- **Checking by itself** (v0.3.2, decision 3). Every scan is also a check
  unless Settings says otherwise, and the answers are remembered, so this
  costs almost nothing.
  - `internal/lastcheck` keeps what each project said in
    `<config>/last-check.json`: the `source.Release` and `resolve.Artifact`
    per entry, with the time. An answer under a day old (`lastcheck.Fresh`)
    is used as it stands; anything older is asked again. Answers nobody has
    wanted for a month are dropped on the next save, so the file can't grow
    by every image that ever left the catalog.
  - A failure is never remembered. A site that was down for a minute must not
    look like bad news until tomorrow.
  - None of it is trusted for a download. `update.Run` resolves the release
    and its checksum again from scratch, so a remembered answer can only ever
    be wrong about what a row *says*, never about what lands on a drive.
  - `check.Memory` is the interface `Report.Online` asks before the network
    and hands each fresh answer to; `inventory.Options.Memory` passes it
    through. Nil means ask about everything.
  - The server picks the memory per scan (`memoryFor`, scanrun.go):
    `askIfDue` is the ordinary scan and obeys the setting, `askAgain` is
    Refresh and asks every project however recently it answered
    (`Answers.Asking()`). The command line's own `check` uses `Asking()` too,
    since typing it is asking, but still writes down what it learns.
  - `Report.CheckedAt` is the oldest answer the report rests on, and the page
    says that rather than when it last drew: a check that reused this
    morning's answers says this morning.
- **Settings** (v0.3.1) is one panel, opened from the top bar and closed with
  Escape, holding everything isoshelf lets a person change. Each setting is
  one entry in `SETTING_GROUPS` (`internal/web/static/settinglist.js`): its name,
  a line saying what it does, the words a search should find it by, and the
  control. The search box matches all of those, so nothing has to be listed
  twice. Settings and the details panel share a place on the screen, so
  opening one closes the other.
  - The answers live in `<config>/ui.json`, which `internal/settings` owns.
    The web server keeps no second copy of the fields: when it did, a switch
    flicked on the page erased what the command line had written. `Load`
    cleans anything it doesn't understand and carries the pre-v0.3.1
    `replace_action` over to `old_files`.
  - `POST /api/settings` takes one answer at a time - every field is a
    pointer, so absent means "leave it alone" - and answers with the whole
    page state. `{"reset": true}` puts the choices back to their defaults and
    keeps what isn't a choice: the folder in use, the pinned folders and
    where records are kept.
  - How it looks is four attributes on `<html>`: `data-theme`,
    `data-contrast`, `data-text` and `data-motion`. The colors are written
    once with `light-dark()`, so a theme is `color-scheme` and nothing else,
    and sizes are in `rem` against one number on `:root`, so "larger text"
    raises all of them together. A copy of the answers is kept in the
    browser's localStorage as well, so the page opens in the right colors
    instead of changing under the reader a moment later; isoshelf is still
    the one that remembers.
  - The panel is redrawn only when what it shows changes (`settingsKey`),
    because the page asks how downloads are doing twice a second and a redraw
    would take the focus out of the control someone is using.
- The folder's free space (`internal/space`) sits next to the scan time. It is
  read before the server's lock is taken and cached for five seconds, because
  the page polls twice a second during a scan and a NAS answers over the
  network. A folder that won't say leaves the line out.
- A row's links (download page, website, forum, release notes), its Update
  button and Remove live in the details panel; the "…" menu the rows used to
  carry went with the v0.3.0 redesign.
- The page is one script per part, all plain scripts sharing the same names
  and loaded in the order `index.html` lists them: `app.js` (what isoshelf has
  said, how the page asks, what gets drawn, the wiring), `images.js` (statuses,
  filters, rows), `summary.js` (the line saying what wants doing, and the
  question a container asks once), `details.js` (the panel), `checklist.js`
  (the checklist), `downloads.js` (the queue), `actions.js` (updating,
  removing), `catalog.js` (the Add images tab), `identify.js` (What is this?),
  `missing.js` (getting a missing image back, or not expecting it),
  `origin.js` (where each file came from, and Check it), `fromserver.js`
  (copying from your server in Add images), `dismiss.js` (dismissing an
  update for a while or for good),
  `folders.js` (the chooser), `archive.js`, `settings.js` (the Settings
  panel), `settinglist.js` (what each setting is), `access.js` (sign-in and
  sharing), `records.js` (where a folder's records live), `upload.js`
  (dragging a file onto the page, or picking one), `selfupdate.js` (isoshelf
  updating its own program) and `report.js` (the
  prefilled bug report). They were one 2,544-line
  `app.js` until v0.3.1; the split is what keeps changing one corner from
  meaning reading all of it.
- Logos: 21 ship in `internal/web/static/logos` (Simple Icons, CC0), refreshed
  with `go run ./internal/web/logos/fetch`. `/logo/{slug}` serves those, then
  ones fetched earlier from `<config>/logos`, then fetches from the CDN once
  and remembers misses. Entries without one get colored initials, drawn from
  the name.
- **Archive** lists files still in `.isoshelf/removed`, with Restore;
  **History** lists images that have left, with Download again for catalog
  ones.
- Downloads (`queue.go`): Add, Update, Update all and Download again join a
  queue that runs one at a time, and the queue drains into one rescan. The
  page shows it in a bar along the bottom (Steam-like): the running download
  with speed and time left, the waiting ones with up/down/next/remove buttons
  and drag and drop, and the finished ones with Try again. Each Add or Update
  button says Queued #n, Downloading n% or Added. Finished jobs live in memory
  only (last 20). Because the page polls twice a second for as long as
  downloads run, it redraws in full only when something other than progress
  and free space changed (`drawnKey`), so open menus and keyboard focus
  survive.
- Static files are embedded, so restart the server after changing them. To
  preview while developing: `go run ./cmd/isoshelf ui --port 8765 --no-browser <folder>`
  and open the printed link (with `localhost` or `127.0.0.1`).

### Status

Where things stand and what comes next lives in [STATUS.md](STATUS.md).

## Sample drive (real filenames - use as scanner test fixtures)

```
Atlas_v0.5.2.iso                           # archival: Atlas no longer ships ISOs
Core-current.iso                           # Tiny Core, 32-bit, fixed name
CorePlus-current.iso                       # Tiny Core, 32-bit, fixed name
TinyCore-current.iso                       # Tiny Core, 32-bit, fixed name
HBCD_PE_x64.iso                            # Hiren's BootCD PE, manual, fixed name
FydeOS_for_PC_iris_v22.0-io-stable.iso     # FydeOS, manual; user renamed from .bin
FydeOS_for_PC_iris_v22.0-SP1-io.bin        # FydeOS, not bootable -> Make bootable (.img)
bazzite-deck-gnome-stable-live-amd64.iso   # Bazzite, fixed name
cachyos-desktop-linux-260308.iso           # CachyOS Desktop, version 260308
Win11_English_x64.iso                      # Windows 11, manual, no version in name
Win11_23H2_English_x64v2.iso               # Windows 11 23H2, manual
Windows10.iso                              # Windows 10, manual
Windows.iso                                # unrecognized by name; Assign finds it
MX-21.3_x64.iso                            # MX Linux, 64-bit track
MX-23.3_x32.iso                            # MX Linux, 32-bit track
manjaro-xfce-23.0.4-231015-linux65.iso     # Manjaro Xfce 23.0.4
netboot.xyz.iso                            # netboot.xyz, fixed name
netboot.xyz-multiarch.iso                  # netboot.xyz multiarch, fixed name
clonezilla-live-20231102-mantic-amd64.iso  # Clonezilla (Ubuntu-based)
CentOS-7-i386-Minimal-2009.iso             # archival, EOL, 32-bit
q4os-5.5-i386-instcd.r1_x32.iso            # Q4OS 5.5, 32-bit
batocera-5.25-x86-20200310.img             # Batocera 5.25, 32-bit
pop-os_22.04_amd64_intel_56.iso            # Pop!_OS 22.04 Intel/AMD, build 56
TrueNAS-SCALE-24.10.2.2-HexOS.iso          # HexOS installer (manual), not stock TrueNAS
linuxmint-22.3-cinnamon-64bit.iso          # Linux Mint 22.3 Cinnamon
notes.txt                                  # not an image: ignore
```

## Working conventions

- `go build ./...`, `go vet ./...` and `go test ./...` must pass before each
  commit.
- Tests never touch real disks (temp dirs only) and never hit the live network.
  `internal/remote/remotetest` replays responses recorded from the real sites.
  `go run ./internal/remote/remotetest/record` (the only thing that goes
  online) resolves every catalog entry live, checks that each downloadable
  image itself answers (a HEAD request, or a one-byte range where HEAD is
  refused), reports failures and redirects, and re-records everything; run it
  after changing the catalog. `TestEveryEntryRecorded` fails when an entry has
  no recording. Add `-sizes` to print the measured `size =` lines for the
  catalog.
- Never claim more than was checked. "All entries resolve" once meant only
  that their checksum files did, and Kali's torrent-only live image slipped
  through that way.
- Small commits with clear messages. Update this file when a decision changes.
- Explain in plain language any step the maintainer has to do by hand
  (installing tools, committing, pushing, releasing).

## Prior art (read for ideas, don't copy code)

- Super ISO Updater (Python, GPL-2.0-or-later): knows where many distros publish
  images and checksums. GPL code can't be copied into this MIT project.
- Ventoy Depot (Python, MIT): similar safety model, on-drive state, hash-bound
  assignments.
