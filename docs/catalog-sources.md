# Catalog sources: research notes

Why each entry in `internal/catalog/default.toml` is set up the way it is.
Sources were checked by hand on 2026-09-16 and 2026-09-17.

To check that every entry still works, run this from the repository root:

```bash
go run ./internal/remote/remotetest/record
```

It resolves every entry against the live sites, lists the ones that fail,
prints a note for every redirect, and saves the responses the tests replay.
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
| Kali installer / Purple / live | listing `kali.download/base-images/` | `kali-{version}/SHA256SUMS` | `cdimage.kali.org` redirects to `kali.download`. |
| Parrot Home / Security | listing `deb.parrot.sh/parrot/iso/` | `{version}/signed-hashes.txt` (clearsigned, md5/sha256/sha512) | Image downloads redirect to a CDN, which is fine for image bytes. |
| Qubes OS | listing `ftp.qubes-os.org/iso/` (rc/beta excluded) | `{file}.DIGESTS` (clearsigned, four algorithms) | |
| SystemRescue | listing of the Download page | `fastly-cdn.system-rescue.org/releases/{version}/{file}.sha256` | The releases folder has no index (403). |
| Rescuezilla (64-bit) | github `rescuezilla/rescuezilla` | asset digest | Each release ships builds on several Ubuntu bases; the catalog follows the 24.04 LTS (`noble`) build. |
| netboot.xyz, multiarch | github `netbootxyz/netboot.xyz` | asset digest | |
| CentOS 7 (32/64-bit, Minimal/Everything) | endoflife `centos`, cycle `7` | `vault.centos.org/…/sha256sum.txt` | 64-bit has a newer `2207-02` respin; 32-bit only `2009`. |

## Check-only entries

Update and EOL checks work, but there's nothing to download yet.

- **MX Linux** (x64, x32): endoflife `mxlinux`. Files are on SourceForge, which
  redirects to mirrors, so it's unclear where the official HTTPS checksums
  are. MX 25 renamed files to `MX-25.2_Xfce_x64.iso`; monthly respins add the
  month (`MX-23.1_November_x64.iso`). No 32-bit MX 25 ISO, so x32 is pinned to
  23.
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

Not researched yet. Each one gets checked live before it goes in.

- **Desktop:** Kubuntu, Xubuntu, Linux Mint MATE and Xfce, LMDE, Fedora
  Workstation and KDE, Debian live, Zorin OS, elementary OS, openSUSE Tumbleweed
  and Leap, Manjaro KDE and GNOME, EndeavourOS, Arch Linux.
- **Trending:** more Bazzite variants (KDE, desktop, NVIDIA), Nobara, Aurora,
  Bluefin, NixOS.
- **Server and homelab:** Rocky Linux, AlmaLinux, OPNsense, Talos Linux,
  Proxmox Mail Gateway and Datacenter Manager.
- **Rescue and tools:** GParted Live, Clonezilla stable, Memtest86+, ShredOS.
