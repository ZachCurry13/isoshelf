# isoshelf

Working name; rename freely. An app (Windows + Linux, macOS later) that
inventories, update-checks, downloads and verifies the bootable images on a
Ventoy drive or in any image folder (NAS share, Proxmox ISO storage). It can be
installed on a PC, run portably from the drive itself, or (later) run as a
server in Docker/LXC. The UI is a web page served by the app. Independent
project, not affiliated with Ventoy.

## Hard rules

- Never touch partitions, bootloaders, or Ventoy's `/ventoy` folder. Only work
  with image files inside the folder the user picks.
- Never delete or overwrite anything the user hasn't chosen to replace. Each
  track has a "replace old file" checkbox, on by default; off = keep old files.
  Replace = download -> verify -> rename into place -> only then delete the old
  file(s) of that same track.
- A download without a published checksum ("unverified") never replaces
  anything on its own: the old file stays until the user confirms that item.
- Only ever delete image files inside the folder the user picked: recognized
  images, and unrecognized ones such as a renamed Windows ISO (decided
  2026-09-17, replacing "only files matched to a catalog entry"). Never other
  files (notes, archives, anything without an image extension or image
  content), and never anything outside that folder.
- Every delete asks first, per file, and offers both: move aside into
  `<target>/.isoshelf/removed/` (instant and undoable; the space is freed when
  the user empties it, and isoshelf shows how much it holds), or delete now.
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

## Stack

- Go. One binary per OS, no runtime dependencies. Everything builds with
  `CGO_ENABLED=0`.
- UI: a web UI served by the same binary (`internal/web`), with HTML/CSS/JS
  embedded via `embed` and no Node build step. Only `internal/web` knows about
  HTTP handlers and HTML.
  - Desktop and portable: listen on `127.0.0.1` and open the browser. Guard with
    a random per-launch token and check `Host`/`Origin` headers (blocks DNS
    rebinding and cross-site requests).
  - Server (`isoshelf serve`): configurable address and port, reached like other
    homelab apps at `http://<nas-ip>:<port>`. Requires a login.
- macOS later: same code, but needs a Mac or macOS CI runner, and Apple signing
  and notarization for a smooth first launch.
- OpenPGP: `github.com/ProtonMail/go-crypto` (not deprecated `x/crypto/openpgp`).
- Free space and filesystem type: `golang.org/x/sys`.
- Archives (pure Go only): stdlib `compress/gzip` and `archive/zip`,
  `github.com/ulikunitz/xz`, `github.com/bodgit/sevenzip`.
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
   fetches the checksum manifest, finds the exact filename by regex (or in the
   directory index at `base`, when there's no manifest or its name uses
   `{file}`). A manifest that redirects to another host is refused. Output:
   `Artifact{Filename, URLs, Size, Checksum}`.
3. **Verifier** (`internal/verify`) - GNU (`hash  file`) and BSD
   (`SHA256 (file) = hash`) manifests; MD5/SHA-1 count as weak integrity only.
   Signature shapes: detached over manifest, clearsigned manifest, detached over
   the image. Keys are armored files with fingerprints pinned in the catalog.
4. **Fetcher** (`internal/fetch`) - `.part` files in `<target>/.isoshelf/partial/`
   plus a sidecar (URL, ETag, offset, SHA-256 state), resume via `Range` +
   `If-Range`, backoff on timeouts/5xx, no retry on 404, GitHub rate-limit
   reset times, mirrors only when a checksum can prove the bytes. A checksum
   mismatch deletes the partial file and places nothing. `BeforePlace` runs
   after verification and just before the rename, which is how the old file is
   moved aside for images whose filename never changes. Still to do: a queue
   (2 concurrent, 1 per host) for several downloads at once.

5. **Update** (`internal/update`) - resolves the entry again, downloads,
   places, records it in state, then keeps, moves aside or deletes the old
   files of that track. It also removes files the user no longer wants, and
   empties `<target>/.isoshelf/removed/`.

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
  - `ventoy` (default): recursive; bootable = `.iso .wim .img .vhd .vhdx .efi`.
  - `proxmox`: top level only; bootable = `.iso .img`, case-insensitive
    (Proxmox's `$ISO_EXT_RE_0`; it lists nothing else and ignores subfolders).
    Suggested automatically when the path ends in `template/iso`.
- Downloads and fix-up output are staged in `<target>/.isoshelf/`, so the final
  rename never crosses filesystems (NAS shares, or a portable app on a USB drive
  managing a NAS folder).
- State lives in `.isoshelf/state.json` in the chosen folder: profile, placed
  files (entry, version, filename, SHA-256, source URL, date), the per-track
  keep/replace choice, manual assignments for renamed files, and scan history
  (last 100 scans). A random `target_id` tells targets apart when drive letters
  change. File records stay valid while size and modification time are
  unchanged; a changed file loses its hash and assignment.
- History and the usual set are mirrored to `<config>/targets/<target_id>.json`
  so they survive a dead drive (not in portable mode; offer "Export usual set" there instead).
- Usual set = starred entries + entries seen in at least 2 of the last 10
  scans. "Missing" means
  missing from the usual set, not from the whole catalog. Rebuild offers the
  usual set as a preset.
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
  `isoshelf-windows-amd64.exe`, `isoshelf-linux-amd64`, and a `portable` marker
  file) that the user copies onto the drive.
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
- Code: `internal/catalog`. The file starts with `schema = 1`.
- One `[[entry]]` = one track:
  - `id` (lowercase, digits, dashes), `name`, `arch` (`x86_64`, `x86`,
    `arm64`, `arm` (32-bit), `multi`).
  - `match`: regex over the whole filename. Needs a `version` group, unless
    `fixed_name = true` (then it must not have one) or the source is manual
    (then it's optional).
  - `samples`: at least one real filename; each must match its own entry only.
  - Optional: `fixed_name`, `page`, `fixup` (`extract`, `convert`,
    `rename:<ext>`), `known_hashes` (SHA-256).
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
  can add images that aren't on their target yet. It has 60 entries (the
  sample drive and the maintainer's Proxmox folder). Add more in batches,
  checking each one live before it goes in. Why each entry is set up the way
  it is, and the wish list: `docs/catalog-sources.md`.
- Every catalog URL is the final address: tests can't replay redirects, and a
  checksum file that redirects to another host is refused.
- Later: one file per distro under `catalog/`, validated in CI, plus a weekly
  workflow that resolves every entry and opens an issue when one breaks.

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

**v0.2** - downloads and the actions around them:
1. Downloads: resumable fetch, checksum verification, place into the target.
   An Update button per image plus "Update all". *Done.*
3. Delete: remove images the user no longer wants. *Done.*
2. Assign: suggest what an unrecognized file is and confirm it (below).
4. Adding catalog images that aren't on the target (the "Add" button).
5. Installing older versions with a hold (below), and Make bootable fix-ups.
6. Still open: a CLI `isoshelf update` command (the web UI has it), a queue
   for several downloads at once, and OpenPGP signature checking.
**v0.3** - rebuild and repair modes.
**Later** - server mode (below); macOS build.
**Releases** - GitHub Actions matrix (Windows + Linux) on `v*` tags; attach
binaries, the portable zip, and `SHA256SUMS` to the release.

### Assign: suggesting what an unrecognized file is

For files the catalog doesn't match by name, such as `Windows.iso` from the
Media Creation Tool:

- Read the ISO 9660 volume label (32 bytes at 0x8028) and creation date from
  the primary volume descriptor: Windows images carry labels like
  `CCCOMA_X64FRE_EN-US_DV9`, Ubuntu ones `Ubuntu 24.04.3 LTS amd64`.
- Rank candidates by label, by known_hashes and published checksums that match
  the file's hash, by content kind, and by name similarity.
- Show the best guesses with the reason ("its label says Windows 11"), and let
  the user confirm or pick any catalog entry. Never assign silently.
- A confirmed assignment is stored in the target's state against the file's
  size and hash, so later scans recognize it and check it for updates.

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

- Release builds embed their version (`-ldflags "-X main.version=v0.1.0"`).
  Development builds report `dev` and never check.
- On start (and daily in server mode), ask the GitHub API for the latest
  release of `ZachCurry13/isoshelf`. If it's newer, show a notice with a link
  to the release notes: one line on stderr in the CLI, a banner in the web UI.
  The check can be turned off; the last check time lives in the config folder.
- Only a notice for now: users download the new build themselves (portable:
  replace the files on the drive; Docker: pull the new image). A later
  "install update" must verify the release's `SHA256SUMS` first.
- The repo must be public by the first release, or both the check and
  downloads fail for users.

### Server mode (later, not started)

Run isoshelf unattended on a NAS or hypervisor and manage it from a browser.

- `isoshelf serve`: the same binary and web UI as the desktop. Ship a Docker
  image (static binary + CA certificates). That also covers TrueNAS apps, which
  are Docker-based. Also document a Proxmox LXC with the ISO storage
  bind-mounted.
- UI: browse the catalog and add tracks not on the shelf, update, the per-track
  keep/replace checkbox, history.
- Scheduler: check daily by default and download verified updates, replacing
  old files where the checkbox says so. Unverified updates wait for the user.

### CLI (`cmd/isoshelf`)

- `scan [flags] [folder]` (offline) and `check [flags] [folder]` (online);
  flags may come before or after the folder: `--profile ventoy|proxmox`
  (saved in state), `--json`, `--catalog FILE`, `--no-hash`, `--no-update-check`.
- The folder defaults to the drive in portable mode and is required otherwise.
- Catalog: `--catalog`, else `<config>/catalog.toml` if present, else built in.
- Env: `GITHUB_TOKEN` (rate limit), `ISOSHELF_NO_UPDATE_CHECK`.
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
  (keep_old, starred). One scan or check runs at a time, in the background;
  the page polls `/api/state` while it runs. Changes are refused while one runs.
- The folder picker lists subfolders through `/api/browse` (browsers can't see
  the computer's folders), with drives or mount points and recent folders
  (from the mirrors; none in portable mode). The last folder is remembered in
  `<config>/ui.json`.
- A filled star means the user starred the image; the replace switch shows only
  for entries that can download.
- Static files are embedded, so restart the server after changing them. To
  preview while developing: `go run ./cmd/isoshelf ui --port 8765 --no-browser <folder>`
  and open the printed link (with `localhost` or `127.0.0.1`).

### Where we stopped (2026-09-17)

v0.1 is done (catalog of 60 entries, scanner, state, sources, CLI, web UI) and
v0.2 has downloads and removal working, tried end to end against the real
netboot.xyz release. Next, in order:

- Assign: suggest what unrecognized files are (volume label, hashes, name) and
  confirm, so files like `Windows.iso` can be recognized and checked.
- The "Add" button: download a catalog image the folder doesn't have yet.
- `isoshelf update` on the command line, to match the web UI.
- Older versions with a hold; Make bootable fix-ups; signature checking.
- Before a release: GitHub Actions (build, vet, test on Windows and Linux;
  release binaries, portable zip, SHA256SUMS), and the repository must be
  public for the update notice and downloads.

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
Windows.iso                                # unrecognized -> needs Assign
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
  online) resolves every catalog entry live, reports failures and redirects,
  and re-records everything; run it after changing the catalog.
  `TestEveryEntryRecorded` fails when an entry has no recording.
- Small commits with clear messages. Update this file when a decision changes.
- Explain in plain language any step the maintainer has to do by hand
  (installing tools, committing, pushing, releasing).

## Prior art (read for ideas, don't copy code)

- Super ISO Updater (Python, GPL-2.0-or-later): knows where many distros publish
  images and checksums. GPL code can't be copied into this MIT project.
- Ventoy Depot (Python, MIT): similar safety model, on-drive state, hash-bound
  assignments.
