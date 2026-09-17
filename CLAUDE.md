# isoshelf

Working name; rename freely. A desktop app (Windows + Linux, macOS later) that
inventories, update-checks, downloads and verifies the bootable images on a
Ventoy drive. It can be installed on a PC or run portably from the drive itself.
Independent project, not affiliated with Ventoy.

## Hard rules

- Never touch partitions, bootloaders, or Ventoy's `/ventoy` folder. Only work
  with image files inside the folder the user picks.
- Never delete or overwrite anything without explicit per-item confirmation.
  Replace = download -> verify -> rename into place -> only then delete the old
  file, and only if the user chose that.
- Only ever delete files the scanner matched to a catalog entry.
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

- Go. One binary per OS, no runtime dependencies.
- UI: Fyne, imported only by `internal/ui`. Everything else must build with
  `CGO_ENABLED=0`, so it can be tested headless and reused by the CLI.
- Fyne needs CGO: on Windows a MinGW-w64 gcc (e.g. via MSYS2). Only needed once
  work on `internal/ui` starts.
- macOS later: same code, but needs a Mac or macOS CI runner, and Apple signing
  and notarization for a smooth first launch.
- OpenPGP: `github.com/ProtonMail/go-crypto` (not deprecated `x/crypto/openpgp`).
- Free space and filesystem type: `golang.org/x/sys`.
- Archives (pure Go only): stdlib `compress/gzip` and `archive/zip`,
  `github.com/ulikunitz/xz`, `github.com/bodgit/sevenzip`.
- Catalog: TOML, regexes in single-quoted literal strings.

## Architecture

Every catalog entry runs through four stages:

1. **Source** (`internal/source`) - "what's the latest version?"
   - `endoflife`: endoflife.date API v1,
     `https://endoflife.date/api/v1/products/{product}/`. `channel` is
     `latest`, `lts`, or a pinned cycle. Flavors reuse the parent product
     (Kubuntu uses `ubuntu`).
   - `github`: newest release whose tag matches a regex. File comes from the
     release assets (verify with the per-asset SHA-256 `digest` in the REST
     API), or the release is only a version label and the file lives elsewhere.
   - `listing`: a checksum file, a mirror's directory index, or a JSON endpoint;
     version extracted by regex.
   - `static`: version + URL pinned by hand in the catalog.
   - `manual`: inventory only; optional known-good SHA-256 list; "open download
     page" action.
2. **Resolver** (`internal/resolve`) - expands `{cycle}`/`{version}` templates,
   fetches the checksum manifest, finds the exact filename by regex. Output:
   `Artifact{Filename, URLs, Size, Hash}`.
3. **Verifier** (`internal/verify`) - GNU (`hash  file`) and BSD
   (`SHA256 (file) = hash`) manifests; MD5/SHA-1 count as weak integrity only.
   Signature shapes: detached over manifest, clearsigned manifest, detached over
   the image. Keys are armored files with fingerprints pinned in the catalog.
4. **Fetcher** (`internal/fetch`) - queue (2 concurrent, 1 per host), `.part`
   files plus a sidecar (URL, ETag, offset), resume via `Range` + `If-Range`,
   hash state saved with the partial file, backoff on timeouts/5xx, no retry on
   404, show GitHub rate-limit reset time, optional read-back verification.

Images whose filename never changes (`bazzite-...-stable-...iso`,
`Core-current.iso`, `HBCD_PE_x64.iso`): "update available" means the published
checksum differs from the checksum recorded in drive state. A new GitHub tag
alone is not an update.

## Drive state and scanning

- State lives in `.isoshelf/state.json` in the chosen folder: placed files
  (entry, version, filename, SHA-256, source URL, date), manual assignments for
  renamed files, and scan history.
- History and the usual set are mirrored to the OS config dir so they survive a
  dead drive (not in portable mode; offer "Export usual set" there instead).
- Usual set = entries kept across scans + anything starred. "Missing" means
  missing from the usual set, not from the whole catalog. Rebuild offers the
  usual set as a preset.
- Scanner is recursive; skips `.Trash-*`, `ventoy/`, `.isoshelf/` and the
  portable app folder, but reports how much space `.Trash-*` folders use.
- Images Ventoy lists: `.iso .wim .img .vhd .vhdx .efi`. Flag files that match
  a catalog entry but have another extension (e.g. `.bin`) and offer
  "Make bootable".
- First scan hashes fixed-filename images in the background (cancellable) and
  records the results.
- Several files for one entry are grouped, with a "keep newest" action.
- Statuses: up to date, update available, missing, manual, unverified, checksum
  mismatch, EOL (endoflife.date `isEol`), not bootable, unrecognized, check
  failed (showing the real error).

## Portable mode

- Releases include a portable folder (`isoshelf/` with
  `isoshelf-windows-amd64.exe`, `isoshelf-linux-amd64`, and a `portable` marker
  file) that the user copies onto the drive.
- Marker next to the executable -> portable mode: store everything in that
  folder, use a temp dir on the drive, and default the target to the drive the
  app is running from.
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
- One entry = one track: `id`, `name`, `arch`, `match` (regex with named group
  `version`), `[source]`, `[artifact]` (`base`, `manifest`, `file`, `sig`),
  optional `known_hashes`, `fixup`, `page`.
- Validation: regexes compile, templates are valid, referenced keys exist, each
  sample filename matches exactly one entry.
- Later: one file per distro under `catalog/`, validated in CI, plus a weekly
  workflow that resolves every entry and opens an issue when one breaks.

## Milestones

**v0.1 - read-only** (the only writes are to `.isoshelf/`)
1. Catalog loader + validation.
2. Scanner + filename matching + content sniffing, with table tests built from
   the sample drive.
3. Drive state + usual-set history, including portable-mode storage.
4. Sources: `endoflife`, `github`, `listing`, `manual` (check only).
5. CLI: `isoshelf scan <folder>` and `isoshelf check <folder>` print a status
   table; `--json` for machine output.
6. Fyne window showing the same table (needs the CGO toolchain).

**v0.2** - downloads, verification, replace flow, Make bootable fix-ups.
**v0.3** - rebuild and repair modes.
**Later** - macOS build.
**Releases** - GitHub Actions matrix (Windows + Linux) on `v*` tags; attach
binaries, the portable zip, and `SHA256SUMS` to the release.

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
- Tests never touch real disks (temp dirs only) and never hit the live network
  (use `httptest` with recorded responses).
- Small commits with clear messages. Update this file when a decision changes.
- Explain in plain language any step the maintainer has to do by hand
  (installing tools, committing, pushing, releasing).

## Prior art (read for ideas, don't copy code)

- Super ISO Updater (Python, GPL-2.0-or-later): knows where many distros publish
  images and checksums. GPL code can't be copied into this MIT project.
- Ventoy Depot (Python, MIT): similar safety model, on-drive state, hash-bound
  assignments.
