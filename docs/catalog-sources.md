# Catalog sources: research notes

Where each sample-drive image publishes its releases and checksums, checked by
hand on 2026-09-16. Use these notes to fill `internal/catalog/default.toml`, and
re-check anything that looks stale. Nothing here is guaranteed to be up to date.

## Ready to automate

| Image | Source | Checksums |
|---|---|---|
| Linux Mint Cinnamon | endoflife `linuxmint`. Each point release is its own cycle (`22.3`, `22.2`), with no `latest` field. The same product also lists `lmde7`, so the track needs a cycle filter. | Mint's download page links `https://mirrors.kernel.org/linuxmint/stable/{cycle}/sha256sum.txt` and `sha256sum.txt.gpg`. |
| netboot.xyz, netboot.xyz multiarch | github `netbootxyz/netboot.xyz`, tags like `3.0.3`. Assets `netboot.xyz.iso` and `netboot.xyz-multiarch.iso` (also `-arm64`, `-sb`, `-legacy`: other tracks). | Per-asset `digest` in the API. |
| Pop!_OS 22.04 Intel/AMD | listing `https://api.pop-os.org/builds/22.04/intel` (JSON: `build`, `url`, `sha_sum`). Build was 58. endoflife `pop-os` has cycles `24.04`, `22.04`, without `latest`. | `https://iso.pop-os.org/22.04/amd64/intel/{build}/SHA256SUMS` and `SHA256SUMS.gpg`. |
| CachyOS Desktop | listing `https://mirror.cachyos.org/ISO/desktop/` (index of `YYMMDD/` folders). `cdn77.cachyos.org` and `iso.cachyos.org` returned 404. | `{version}/cachyos-desktop-linux-{version}.iso.sha256` (GNU format), plus `.sig`. |
| Bazzite deck GNOME (fixed name) | github `ublue-os/bazzite`. Stable tags look like `44.20260916`; unstable ones start with `unstable-`. | `https://download.bazzite.gg/bazzite-deck-gnome-stable-live-amd64.iso-CHECKSUM` (GNU format). |
| CentOS 7 i386 Minimal (archival) | endoflife `centos`, cycle `7`: EOL, latest `7 (2009)`. | `https://vault.centos.org/altarch/7.9.2009/isos/i386/sha256sum.txt` (and `.asc`). |

## Open questions

- **MX Linux** (x64 and x32 tracks): endoflife `mxlinux`. Cycle 25 is at 25.2,
  23 at 23.6, and 21 is EOL. Files are on SourceForge (`/Final/Xfce/`), which
  redirects downloads to mirrors, so it's unclear where the official HTTPS
  checksums are. MX 25 renamed files to `MX-25.2_Xfce_x64.iso` (older:
  `MX-21.3_x64.iso`); `_ahs_` and KDE/Fluxbox builds are other tracks. No
  32-bit MX 25 ISO turned up, so pin the x32 track to cycle `23` until one
  does.
- **Manjaro Xfce**: no endoflife product. `https://manjaro.org/products/download/x86`
  links `download.manjaro.org/xfce/{v}/manjaro-xfce-{v}-{yymmdd}-linux{k}.iso`
  (was 26.1.0). There's no directory index, so the exact filename has to come
  from that page. `-minimal-` builds are another track.
- **Clonezilla alternative (Ubuntu-based)**: no endoflife product. SourceForge
  RSS `https://sourceforge.net/projects/clonezilla/rss?path=/clonezilla_live_alternative`
  lists `{yyyymmdd}-{codename}/clonezilla-live-{yyyymmdd}-{codename}-amd64.iso`.
  No checksum file found yet.
- **Tiny Core** (`Core`, `CorePlus`, `TinyCore` `-current.iso`, 32-bit): the
  official site is `http://` only and publishes MD5 only. `Core-current.iso`
  has no checksum file of its own (only `Core-16.2.iso.md5.txt`). The HTTPS
  copy at `distro.ibiblio.org` is a mirror. Manual for now.

## Manual

No update source. Download pages (all returned HTTP 200 unless noted):

- AtlasOS (archival ISO): `https://atlasos.net/`
- Hiren's BootCD PE: `https://www.hirensbootcd.org/download/`
- FydeOS for PC: `https://fydeos.io/download/`
- HexOS installer: `https://hexos.com/`
- Q4OS: `https://www.q4os.org/downloads2.html` (blocked the scripted check, HTTP 466)
- Batocera (32-bit x86 builds): `https://batocera.org/download`
- Windows 10 and 11: `https://www.microsoft.com/software-download/windows10`
  and `.../windows11` (blocked the scripted check, HTTP 403)
- Tiny Core: `http://tinycorelinux.net/downloads.html`
