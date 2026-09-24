# Catalog sources: research notes

Why each entry in `internal/catalog/default.toml` is set up the way it is.
Sources were checked by hand on 2026-09-16 and 2026-09-17.

To check that every entry still works, run this from the repository root:

```bash
go run ./internal/remote/remotetest/record
```

It resolves every entry against the live sites, checks that each image itself
answers (a HEAD request, or a one-byte range where HEAD is refused), lists the
ones that fail, prints a note for every redirect, and saves the responses the
tests replay. A checksum file can list an image the server doesn't hand out,
which is how Kali's torrent-only live images went unnoticed until the image
check was added.
Catalog URLs should always be the final address (tests can't replay
redirects, and a checksum file that redirects to another host is refused).

## Download entries

| Entry | Source | Checksums | Notes |
|---|---|---|---|
| Ubuntu Desktop / Server LTS | endoflife `ubuntu`, channel `lts` | `releases.ubuntu.com/{cycle}/SHA256SUMS` | LTS files are even-year `.04`; a cycle folder can hold several point releases (and oddities like `24.04.5.1`), so the newest match wins. |
| Linux Mint Cinnamon | endoflife `linuxmint`; cycles filter keeps LMDE out | `mirrors.edge.kernel.org/linuxmint/stable/{cycle}/sha256sum.txt` | Mint's page links `mirrors.kernel.org`, which redirects (301) to the edge address. |
| Debian netinst (64-bit) | endoflife `debian`, latest | `cdimage.debian.org/debian-cd/current/amd64/iso-cd/SHA256SUMS` | The same file lists `debian-edu` and `debian-mac` netinst images, which don't match. |
| Debian 12 DVD (32-bit) | endoflife `debian`, cycle `12` | `cdimage.debian.org/cdimage/archive/{version}.0/i386/iso-dvd/SHA256SUMS` | Debian 13 has no 32-bit PC installer images. |
| Pop!_OS 22.04 / 24.04, Intel and NVIDIA | listing `api.pop-os.org/builds/{release}/{intel,nvidia}` (JSON `build`) | `iso.pop-os.org/{release}/amd64/{channel}/{build}/SHA256SUMS` | The 22.04 Raspberry Pi build is gone from the API (404): manual. |
| CachyOS Desktop | listing `mirror.cachyos.org/ISO/desktop/` | `{version}/cachyos-desktop-linux-{version}.iso.sha256` | `cdn77.cachyos.org` and `iso.cachyos.org` returned 404. |
| Bazzite Deck GNOME (fixed name) | github `ublue-os/bazzite` (unstable/testing are prereleases) | `download.bazzite.gg/…-CHECKSUM` | |
| Alpine Extended / Standard / Virtual | endoflife `alpine-linux` (`alpine` redirects there) | `dl-cdn.alpinelinux.org/alpine/v{cycle}/releases/x86_64/{file}.sha256` | |
| Proxmox VE, Proxmox Backup Server | endoflife `proxmox-ve`, `proxmox-backup-server` (cycles are majors: 9, 4) | `enterprise.proxmox.com/iso/SHA256SUMS` | Also lists arm64 PVE images and older majors; the file pattern keeps the track. |
| TrueNAS CORE 13.0 | listing `download.freenas.org/13.0/STABLE/latest/x64/` | `TrueNAS-{version}.iso.sha256` (BSD format with a build path) | No endoflife product. |
| FreeBSD bootonly, 64- and 32-bit | listing of `download.freebsd.org/releases/{arch}/{arch}/ISO-IMAGES/` | `CHECKSUM.SHA256-FreeBSD-{version}-RELEASE-{arch}` (BSD format) | endoflife `freebsd` orders the 14.x and 15.x branches by date, so it would pick 14.5 over 15.1. No i386 images for 15. |
| pfSense CE installer | listing `atxfiles.netgate.com/mirror/downloads/` | `{file}.sha256` | Published as `.iso.gz`; 2.7.2 is the last CE ISO (newer releases use Netgate's online installer). |
| Ubuntu Core amd64 / Raspberry Pi arm64 | listing `cdimage.ubuntu.com/ubuntu-core/` | `{version}/stable/current/SHA256SUMS` | `.img.xz`. The Intel IoT and armhf Raspberry Pi images only exist for Core 20 (static version). |
| Kali installer / Purple | listing `kali.download/base-images/` | `kali-{version}/SHA256SUMS` | `cdimage.kali.org` redirects to `kali.download`. The live image is check-only (below). |
| Parrot Home / Security | listing `deb.parrot.sh/parrot/iso/` | `{version}/signed-hashes.txt` (clearsigned, md5/sha256/sha512) | Image downloads redirect to a CDN, which is fine for image bytes. |
| Qubes OS | listing `ftp.qubes-os.org/iso/` (rc/beta excluded) | `{file}.DIGESTS` (clearsigned, four algorithms) | |
| SystemRescue | listing of the Download page | `fastly-cdn.system-rescue.org/releases/{version}/{file}.sha256` | The releases folder has no index (403). |
| Rescuezilla (64-bit) | github `rescuezilla/rescuezilla` | asset digest | Each release ships builds on several Ubuntu bases; the catalog follows the 24.04 LTS (`noble`) build. |
| netboot.xyz, multiarch | github `netbootxyz/netboot.xyz` | asset digest | |
| CentOS 7 (32/64-bit, Minimal/Everything) | endoflife `centos`, cycle `7` | `vault.centos.org/…/sha256sum.txt` | 64-bit has a newer `2207-02` respin; 32-bit only `2009`. |

## Check-only entries

Update and EOL checks work, but there's nothing to download yet.

- **Kali Linux live**: the same listing as the installers, for update checks.
  Since 2026.1 Kali offers live images only as torrents: `SHA256SUMS` lists
  `kali-linux-{version}-live-amd64.iso`, but the folder has only the
  `.torrent`, and the image answers 404. isoshelf doesn't use torrents.
- **MX Linux** (x64, x32): endoflife `mxlinux`. Files are on SourceForge, which
  redirects to mirrors, so it's unclear where the official HTTPS checksums
  are. MX 25 renamed files to `MX-25.2_Xfce_x64.iso`; monthly respins add the
  month (`MX-23.1_November_x64.iso`). No 32-bit MX 25 ISO, so x32 is pinned to
  23.
  - Checked 2026-09-23 in a browser: the newest 32-bit ISO is
    `Old/MX-23.6/Xfce/MX-23.6_386.iso` on SourceForge, with a `.sha256`, `.md5`
    and `.sig` beside it - named `_386`, not `_x32`, though the folder's
    README says `_i386`. MX's own download page lists only MX 25, so the
    32-bit entry's `page` links that folder instead, and has to move if a
    23.7 ever appears. The checksum is there, but SourceForge serves it
    through a redirect to a mirror, which the catalog refuses, so the entry
    stays check-only.
- **Manjaro Xfce**: listing of `manjaro.org/products/download/x86/` (the address
  without the trailing slash redirects). No directory index for the exact
  filename.
- **Manjaro ARM Xfce (generic)**: github `manjaro-arm/generic-images`, last
  release 23.02; its assets predate GitHub digests and only have `.sha1` files.
- **Clonezilla (Ubuntu-based)**: SourceForge RSS; no checksum file found.
- **TrueNAS Community Edition (SCALE)**: endoflife `truenas` (about one point
  release behind the download page). Download folders carry a codename
  (`TrueNAS-SCALE-Goldeye/25.10.7/`) that can't be derived from the version.
  The 22.x cycles aren't in endoflife.date, so those files get no EOL flag.
- **Tails**: endoflife `tails`. Checksums are only in
  `tails.net/install/v2/Tails/amd64/stable/latest.json`, and downloads redirect
  to `mirrors.edge.kernel.org`. Supporting that JSON would make it a download
  entry.

## Manual entries

No usable update source. Download pages:

- AtlasOS: `https://atlasos.net/`
- EasyOS: `https://easyos.org/` (release folders are named by codename)
- FydeOS for PC: `https://fydeos.io/download/`
- HexOS installer: `https://hexos.com/`
- Hiren's BootCD PE: `https://www.hirensbootcd.org/download/`
- NiceHash OS: `https://www.nicehash.com/nhos`
- Pop!_OS 22.04 Raspberry Pi: discontinued
- Q4OS: `https://www.q4os.org/downloads2.html`
- Batocera (32-bit x86): `https://batocera.org/download`
- Tiny Core: `http://tinycorelinux.net/downloads.html` (http only, MD5 only;
  the `-current` files have no checksum file of their own)
- Windows 10, 11 and 11 Insider Preview: Microsoft's download pages (they block
  scripted requests with HTTP 403)
- `Windows.iso`, the Media Creation Tool's default name, stays unrecognized on
  purpose: it could be Windows 10 or 11.

## Wish list

Still to look at, in order. First the gaps in DistroWatch's 12-month top 50
(its page-hit ranking, used only as a checklist of what people look for, never
as a source of addresses or checksums), then earlier requests. Each image is
checked on the project's own site before it goes in. The weekly catalog job
works from the top, up to three a week, takes each name off when it's added,
and moves ruled-out ones to "Ruled out" with the reason.

1. AnduinOS
2. PikaOS
3. BigLinux
4. antiX
5. elementary OS
6. Void Linux
7. KDE neon
8. Garuda Linux (its build server's folders didn't list on 2026-09-18)
9. MiniOS
10. TUXEDO OS
11. Puppy Linux
12. AerynOS
13. PCLinuxOS
14. SparkyLinux
15. ZimaOS
16. Devuan
17. Mageia
18. Linux Lite
19. Solus
20. pearOS
21. ChromeOS Flex (Google's own installer; probably link only)
22. Exton
23. CentOS Stream
24. HackerOS
25. Linuxfx
26. KDE Linux
27. Linux Mint MATE
28. openSUSE Leap
29. Manjaro KDE and GNOME
30. Bazzite's KDE, desktop and NVIDIA variants
31. Aurora
32. Bluefin
33. OPNsense
34. Talos Linux
35. Proxmox Datacenter Manager
36. LibreELEC
37. DietPi
38. Umbrel
39. Memtest86+
40. ShredOS

### Ruled out

Nothing yet.

## Entry metadata

Besides sources, every entry carries what the page needs to show it: a
`category` (desktop, gaming, server, boards, security, rescue, windows,
other; see Kinds below), a `family` so
one product's tracks group together, `site` and `forum` links, and an `icon`
(a [Simple Icons](https://simpleicons.org) name) with its `icon_color`. The links of
the first 72 entries were checked on 2026-09-17, and those added since on the
day they went in; a few sites answer scripted requests with 403
or 406 but are fine in a browser (MX Linux, Kali's forums, Q4OS, Microsoft).

Twenty-one projects have a logo in Simple Icons and ship inside isoshelf;
refresh them with `go run ./internal/web/logos/fetch`. The rest show colored
initials. Entries added later, or from a user's own catalog, have their logo
fetched once at runtime and kept in the settings folder.

## What discs say about themselves

isoshelf reads the ISO 9660 primary volume descriptor of every image it scans
(`internal/sniff`), and `internal/identify` uses it to suggest what an
unrecognized file is. These labels were read off real images in a Proxmox ISO
folder on 2026-09-17, and they are why the guesser weighs labels heavily but
never trusts them alone:

| Label | Image | Worth |
|---|---|---|
| `Ubuntu 22.04.3 LTS amd64`, `Ubuntu-Server 22.04.3 LTS amd64` | Ubuntu | product, edition and version |
| `CentOS 7 i386`, `CentOS 7 x86_64` | CentOS 7 | product and architecture, but Minimal and Everything are identical |
| `Pop_OS 22.04 amd64 Nvidia`, `Parrot home 6.2`, `Q4OS_5.5_Aquarius` | as named | product and version |
| `Kali Linux amd64 1`, `Debian 12.6.0 i386 1` | Kali, Debian | the trailing 1 is the disc number, not a version |
| `alpine-ext 3.19.0 x86_64`, `QUBES-R4-2-0-X86-64` | Alpine, Qubes | product and version, in the project's own spelling |
| `Rescuezilla`, `TRUENAS`, `MX-Live`, `MXLIVE`, `Core`, `TinyCore` | as named | product only |
| `PVE` | Proxmox VE | too short to match a name; the filename carries it |
| `ISOIMAGE` | TrueNAS SCALE | nothing at all: the xorriso default |
| `CCCOMA_X64FRE_EN-US_DV9` | Windows 10 and 11 retail | Microsoft media, but not which Windows |
| `ESD_ISO` | Media Creation Tool output | nothing at all |

Two other fields do more work than the label in places: `publisher`
("THE FREEBSD PROJECT", "IXSYSTEMS INC.", "MICROSOFT CORPORATION") and the
build time. Two copies of one download carry the same build time to the
second, which is how `Windows.iso` in that folder was recognized: same size as
`Windows11.iso`, same build time, so it is the same image under another name.
The `preparer` field is ignored on purpose, because it names the build tool
(xorriso, mkisofs, IMAPI2) rather than the product.

## Added 2026-09-18

Checked live the same day, and all of them resolve in a full recorder run.

| Entry | Source | Checksums | Notes |
|---|---|---|---|
| Kubuntu LTS, Xubuntu LTS | endoflife `ubuntu`, channel `lts` | `cdimage.ubuntu.com/{flavour}/releases/{cycle}/release/SHA256SUMS` | The flavours follow Ubuntu's cycles. Xubuntu's folder also holds "minimal" images, which are a separate track. |
| Linux Mint Xfce | endoflife `linuxmint`, cycles filter keeps LMDE out | `mirrors.edge.kernel.org/linuxmint/stable/{cycle}/sha256sum.txt` | Same shape as the Cinnamon entry. |
| LMDE | listing of `linuxmint/debian/sha256sum.txt` | the same file | endoflife.date files LMDE under `linuxmint` with cycles like `lmde7`, which don't give the plain number the filenames use. The checksum file lists every LMDE released, so the highest wins. |
| Debian Live (GNOME, KDE) | endoflife `debian`, latest | `cdimage.debian.org/debian-cd/current-live/amd64/iso-hybrid/SHA256SUMS` | endoflife says 13.7 and the files say 13.7.0, so the pattern uses `{cycle}\.\d+\.\d+`. The file lists `.iso.contents` and `.iso.log` beside each image; matching the whole name keeps them out. |
| Fedora Workstation, Fedora KDE | listing of the release folder | `Fedora-{edition}-{version}-x86_64-CHECKSUM` | The checksum file is named after the compose (`44-1.7`), which no version source gives, so the folder is read instead. **The release number is pinned in two URLs and has to be raised by hand when a new Fedora ships.** `dl.fedoraproject.org` is the master and doesn't redirect; `download.fedoraproject.org` sends you to a mirror. |
| Arch Linux | listing of `geo.mirror.pkgbuild.com/iso/latest/sha256sums.txt` | the same file | Monthly, named by date. The folder also holds an unversioned `archlinux-x86_64.iso` copy, which stays unrecognized on purpose. |
| openSUSE Tumbleweed | listing of `download.opensuse.org/tumbleweed/iso/` | `…-Snapshot{version}-Media.iso.sha256` | Rolling, one snapshot per day. The `-Current` names redirect to whichever snapshot is newest, so the dated name is read from the folder. |
| Clonezilla Live (stable) | listing of `free.nchc.org.tw/clonezilla-live/stable/CHECKSUMS.TXT` | the same file | Clonezilla is developed at NCHC, whose server carries the stable images and checksums without redirecting. The file has MD5, SHA1 and SHA256 sections; the strongest wins. The older `clonezilla-alternative` entry is the Ubuntu-based build and stays check-only. |
| GParted Live | listing of `gparted.org/gparted-live/stable/CHECKSUMS.TXT` | the same file, by absolute URL | Images are on SourceForge, checksums on gparted.org: bytes from a mirror, checksums from the origin. It must be the `/project/gparted/gparted-live-stable/{version}/` path — the short `/gparted/<file>` one serves a "your download is starting" web page, which is how the size check caught it at 143 KB. |

### Entries pinned to a release number

These need a catalog edit when the project ships a new release, because the
checksum file is named after something no version source reports:

- `fedora-workstation` and `fedora-kde`: the folder `…/releases/44/…` in both
  `source.url` and `artifact.base`.

### Sizes

Every downloadable entry carries a `size`, measured with
`go run ./internal/remote/remotetest/record -sizes` (a HEAD request, or a
one-byte range where a server refuses HEAD). It is a hint, not a promise: it
changes with each release, and the page says "about". Re-measure whenever the
catalog is re-recorded. On 2026-09-18 all 60 downloadable entries had one
and every measured size matched; the other 26 of the 86 are manual or
check-only, with nothing to measure.

## Added 2026-09-18, later the same day

| Entry | Source | Checksums | Notes |
|---|---|---|---|
| Rocky Linux, AlmaLinux (minimal) | endoflife `rocky-linux`, `almalinux` | `CHECKSUM` in `…/{cycle}/isos/x86_64/` (BSD format) | The folders also hold `-latest-` copies with no version in the name, which are skipped. |
| Proxmox Mail Gateway | listing of `enterprise.proxmox.com/iso/SHA256SUMS` | the same file | The same checksum file as Proxmox VE and Backup Server. |
| Raspberry Pi OS (desktop, Lite), 64-bit | listing of the dated `images/` folder | `{file}.sha256` next to each image | Written to a card, not booted from a menu, so kept as `.img.xz`. The checksum file is `.sha256`; there are `.sha1` files too. |
| Home Assistant OS (Pi 5, x86-64) | github `home-assistant/operating-system` | asset digest | |
| Omarchy | listing of `omarchy.org` | `iso.omarchy.org/omarchy-{version}.iso.sha256` | |
| Nobara (Official, GNOME, Steam Handheld) | listing of `nobaraproject.org/download.html` | `{file}.sha256sum` | Dated releases. The image server can't be listed, so the exact filename comes from what the listing matched on the download page. The checksum files name the image `./Nobara-…`, which the parser handles. The old address `/download-nobara/` redirects. |
| NixOS (graphical) | endoflife `nixos` | — (check-only) | The `latest-` images redirect to a versioned folder named after a build (`26.05.9989.ecc58f32d106`), and catalog addresses must be final. |

Link-only for now, because checksums aren't on the project's own site:

- **EndeavourOS**: images and checksums are on mirrors; the project's GitHub
  holds an archive of old releases only.
- **Zorin OS Core**: `mirrors.edge.kernel.org/zorinos-isos/{release}/` has a
  `SHA256SUMS.txt`, but Zorin's own download page is built by script, so it
  couldn't be confirmed that Zorin points there.

### Kinds

Entries are grouped by what people use them for: desktop, gaming (handhelds
included), server and homelab, boards (the Raspberry Pi and other
single-board computers), security and privacy, rescue and tools, Windows, and
other. An image written to a Pi's card belongs under boards even when what it
runs is a server, because that's how people look for it.
