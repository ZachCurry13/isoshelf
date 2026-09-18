# Catalog changes

The list of images isoshelf knows updates itself from this repository, separately
from isoshelf itself — so new images arrive without a new release. This is what
changed in that list, and when. For changes to isoshelf itself, see
[CHANGELOG.md](CHANGELOG.md).

Every image here was checked against the project's own site before it went in.
Something missing? [Ask for it](https://github.com/ZachCurry13/isoshelf/issues/new?template=missing-image.yml).

## 2026-09-18

### Added
- **Desktop:** Omarchy, EndeavourOS, Zorin OS Core, NixOS (graphical
  installer), Kubuntu LTS, Xubuntu LTS, Linux Mint Xfce, LMDE, Debian Live
  (GNOME and KDE Plasma), Fedora Workstation, Fedora KDE Plasma, Arch Linux,
  openSUSE Tumbleweed.
- **Gaming and handhelds:** Nobara (Official, GNOME and Steam Handheld).
- **Server and homelab:** Rocky Linux, AlmaLinux, Proxmox Mail Gateway, Home
  Assistant OS for x86-64.
- **Raspberry Pi and other boards:** Raspberry Pi OS (desktop and Lite, 64-bit),
  Home Assistant OS for the Pi 5.
- **Rescue and tools:** Clonezilla Live (stable), GParted Live.

### Changed
- Two new kinds, *Gaming and handhelds* and *Raspberry Pi and other boards*.
  Bazzite and Batocera moved to gaming; the Raspberry Pi builds of Ubuntu Core
  and Pop!_OS, and Manjaro ARM, moved to boards.
- Every image isoshelf can download now says roughly how big it is.
- A few images are marked as popular, from public round-ups of what people run.
- A few carry a note worth knowing first: CentOS 7 (end of life since June
  2024), Windows 10 (out of support since October 2025), AtlasOS (an
  unofficial modification of Windows), and the Windows Insider Preview (a
  build that expires).

### Fixed
- GParted Live downloaded from a SourceForge address that serves a "your
  download is starting" page instead of the image. It uses the real one now.
- Nobara's download page moved; the catalog follows it to the new address.
- Kali Linux live now checks for updates and links to Kali's page instead of
  downloading. Kali offers its live images only as torrents, which isoshelf
  doesn't use. The Kali installer images still download as before.

## 2026-09-17

The first catalog, 60 images: the ones on the drive and the Proxmox folder
isoshelf was first built against. Ubuntu Desktop and Server, Linux Mint
Cinnamon, Debian, Pop!_OS, CachyOS, Bazzite, MX Linux, Manjaro, Alpine, Kali,
Parrot, Qubes, Tails, Proxmox VE and Backup Server, TrueNAS, FreeBSD, pfSense,
Ubuntu Core, CentOS 7, SystemRescue, Rescuezilla, Clonezilla, netboot.xyz,
Hiren's BootCD PE, Tiny Core, Q4OS, Batocera, FydeOS, EasyOS, HexOS, NiceHash
OS, AtlasOS, and Windows 10 and 11.
