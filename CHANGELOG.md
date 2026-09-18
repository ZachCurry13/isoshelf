# Changelog

What changed in each release of isoshelf, newest first.

Version numbers: the middle number rises for new abilities (v0.3.0 will be
installing older versions and fixing files the boot menu can't read); the last
number rises for improvements to what it already does.

## [v0.2.4] - 2026-09-18

### Added
- **Older copies are easy to clear.** When a folder holds more than one
  version of the same image — one downloaded by hand next to one isoshelf
  fetched, say — a line above the list says how many and how much room they
  use, with **Show them** and **Clear them**. Clearing asks once, lists every
  file it means, and offers archive or delete as usual.
- An **Older copies** filter beside Updates and Favorites.
- **isoshelf notices when the folder changes.** Drop a file in with Explorer
  while isoshelf is open, and a line offers to scan again.

### Fixed
- A bad edit could leave the page script unable to start. Tests now check the
  script's shape, that every element it looks for exists, and that every link
  opening a new tab does so safely.

## [v0.2.3] - 2026-09-18

### Added
- **Sort by any column** — status, image, version, latest, file, size, and
  the replace switch. Click again to reverse. Empty cells always sort last.
- **Size has its own column**, so "what's using all the room" is one click.
- **Update all** has its own line above the list, saying how many images
  have updates.

### Changed
- When an update replaces a file, the choice reads **Replace it (frees
  2.3 GB)** or **Archive it (still uses 2.3 GB, undo any time)**, and isoshelf
  remembers which you usually pick. When the drive is too full for both,
  Replace comes first and says why.
- The catalog's "can be downloaded" tick is now a filter by how an image
  updates: downloads and checks, update checks only, or download page only.

### Fixed
- Catalog cards were squashed into four narrow columns. Three wider ones now.

## [v0.2.2] - 2026-09-18

### Added
- The folder chooser shows each drive's name ("Z: — red14") and how many
  images a place holds.
- **Bookmarks** for folders you use often.

### Changed
- **Your images appear straight away.** Checking fixed-name images (like
  `netboot.xyz.iso`) carries on behind the list instead of holding it up. It
  happens once per file and is remembered.

## [v0.2.1] - 2026-09-18

### Added
- A third kind of folder, **Folder of images**, for a NAS share, a downloads
  folder or a stash of card images. Nothing there is called "not bootable",
  and compressed images (`.img.xz`) count. It's the default now; a real Ventoy
  drive is recognized by its `ventoy` folder.
- **Refresh** in the folder chooser, for a drive plugged in after isoshelf
  started.
- **Report a problem** on every image, for downloads that move or break, and
  a link at the bottom of the catalog to ask for an image it doesn't know.
- Seven more images: Rocky Linux, AlmaLinux, Proxmox Mail Gateway, Raspberry
  Pi OS (desktop and Lite), and Home Assistant OS for the Pi 5 and x86.
- A **popular** marker and "Popular first" sorting in the catalog: a
  hand-picked hint from public round-ups, not a rating.

### Fixed
- Catalog revision numbers now always sort in the right order.

## [v0.2.0] - 2026-09-18

The first release.

- Scans a Ventoy drive, a NAS share or Proxmox ISO storage and works out which
  distribution, edition, architecture and version each image is.
- Checks each one for updates, using endoflife.date, GitHub releases, and the
  projects' own download listings.
- Downloads updates, checks them against the checksums the projects publish,
  and only then puts them in place. Interrupted downloads carry on.
- Removes images you don't want, with an archive you can put them back from.
- Works out what unrecognized files are, and lets you name the ones no list
  will ever know.
- Adds images from a catalog of 72, showing their size and the room you have.
- Keeps that catalog current from this repository without a new release.
- Runs as a page in your browser, or from the command line.

[v0.2.4]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.4
[v0.2.3]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.3
[v0.2.2]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.2
[v0.2.1]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.1
[v0.2.0]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.0
