// Package sampledrive lists real filenames from the maintainer's Ventoy drive
// (see CLAUDE.md) and Proxmox ISO folder, for use as test fixtures.
package sampledrive

// File is one file on the sample drive.
type File struct {
	Name string
	// Entry is the catalog entry id it belongs to, or empty if none does.
	Entry string
	// Version is the version captured from the name, if any.
	Version string
	// Content is what the file holds: "iso", "disk" or "text".
	Content string
}

// Files is the sample drive, in the order CLAUDE.md lists it.
var Files = []File{
	{"Atlas_v0.5.2.iso", "atlasos", "0.5.2", "iso"},
	{"Core-current.iso", "tinycore-core", "", "iso"},
	{"CorePlus-current.iso", "tinycore-coreplus", "", "iso"},
	{"TinyCore-current.iso", "tinycore-tinycore", "", "iso"},
	{"HBCD_PE_x64.iso", "hirens-bootcd-pe", "", "iso"},
	{"FydeOS_for_PC_iris_v22.0-io-stable.iso", "fydeos-pc-iris", "22.0", "disk"},
	{"FydeOS_for_PC_iris_v22.0-SP1-io.bin", "fydeos-pc-iris", "22.0-SP1", "disk"},
	{"bazzite-deck-gnome-stable-live-amd64.iso", "bazzite-deck-gnome", "", "iso"},
	{"cachyos-desktop-linux-260308.iso", "cachyos-desktop", "260308", "iso"},
	{"Win11_English_x64.iso", "windows-11-x64", "", "iso"},
	{"Win11_23H2_English_x64v2.iso", "windows-11-x64", "23H2", "iso"},
	{"Windows10.iso", "windows-10", "", "iso"},
	{"Windows.iso", "", "", "iso"}, // Media Creation Tool's default name: could be 10 or 11
	{"MX-21.3_x64.iso", "mx-linux-xfce-x64", "21.3", "iso"},
	{"MX-23.3_x32.iso", "mx-linux-xfce-x32", "23.3", "iso"},
	{"manjaro-xfce-23.0.4-231015-linux65.iso", "manjaro-xfce", "23.0.4", "iso"},
	{"netboot.xyz.iso", "netbootxyz", "", "iso"},
	{"netboot.xyz-multiarch.iso", "netbootxyz-multiarch", "", "iso"},
	{"clonezilla-live-20231102-mantic-amd64.iso", "clonezilla-alternative", "20231102", "iso"},
	{"CentOS-7-i386-Minimal-2009.iso", "centos-7-i386-minimal", "2009", "iso"},
	{"q4os-5.5-i386-instcd.r1_x32.iso", "q4os-instcd-x32", "5.5", "iso"},
	{"batocera-5.25-x86-20200310.img", "batocera-x86", "5.25", "disk"},
	{"pop-os_22.04_amd64_intel_56.iso", "popos-2204-intel", "56", "iso"},
	{"TrueNAS-SCALE-24.10.2.2-HexOS.iso", "hexos-installer", "24.10.2.2", "iso"},
	{"linuxmint-22.3-cinnamon-64bit.iso", "linuxmint-cinnamon", "22.3", "iso"},
	{"notes.txt", "", "", "text"},
}

// ProxmoxFolder is the top level of the maintainer's Proxmox ISO storage on a
// NAS (Z:\proxmox\template\iso), as listed on 2026-09-17. Content is empty for
// files that are neither ISOs nor disk images. The folder also holds a
// subfolder named "Files".
var ProxmoxFolder = []File{
	{"a10Core.img.gz", "", "", ""},
	{"alpine-extended-3.19.0-x86_64.iso", "alpine-extended", "3.19.0", "iso"},
	{"Atlas_v0.5.2.iso", "atlasos", "0.5.2", "iso"},
	{"CentOS-7-i386-Everything-2009.iso", "centos-7-i386-everything", "2009", "iso"},
	{"CentOS-7-i386-Minimal-2009.iso", "centos-7-i386-minimal", "2009", "iso"},
	{"CentOS-7-x86_64-Everything-2207-02.iso", "centos-7-x64-everything", "2207-02", "iso"},
	{"CentOS-7-x86_64-Minimal-2009.iso", "centos-7-x64-minimal", "2009", "iso"},
	{"clonezilla-live-20231102-mantic-amd64.iso", "clonezilla-alternative", "20231102", "iso"},
	{"Core-current.iso", "tinycore-core", "", "iso"},
	{"CorePlus-current.iso", "tinycore-coreplus", "", "iso"},
	{"corepure64.gz", "", "", ""},
	{"debian-12.6.0-i386-DVD-1_x32.iso", "debian-12-i386-dvd", "12.6.0", "iso"},
	{"easy-5.6.1-amd64.img", "easyos", "5.6.1", "disk"},
	{"FreeBSD-14.0-RELEASE-amd64-bootonly.iso", "freebsd-bootonly-amd64", "14.0", "iso"},
	{"FreeBSD-14.0-RELEASE-i386-bootonly.iso", "freebsd-bootonly-i386", "14.0", "iso"},
	{"HBCD_PE_x64.iso", "hirens-bootcd-pe", "", "iso"},
	{"kali-linux-2023.4-installer-purple-amd64.iso", "kali-installer-purple-amd64", "2023.4", "iso"},
	{"LoadMaster-VLM-7.2.56.0.21331.RELEASE-Linux-KVM-XEN.disk", "", "", ""},
	{"Manjaro-ARM-xfce-generic-23.02.img", "manjaro-arm-xfce-generic", "23.02", "disk"},
	{"manjaro-xfce-23.0.4-231015-linux65.iso", "manjaro-xfce", "23.0.4", "iso"},
	{"MediCat.USB.v21.12.7z", "", "", ""},
	{"MX-21.3_x64.iso", "mx-linux-xfce-x64", "21.3", "iso"},
	{"MX-23.1_November_x64.iso", "mx-linux-xfce-x64", "23.1", "iso"},
	{"MX-23.1_x64.iso", "mx-linux-xfce-x64", "23.1", "iso"},
	{"MX-23.3_x32.iso", "mx-linux-xfce-x32", "23.3", "iso"},
	{"netboot.xyz-multiarch.iso", "netbootxyz-multiarch", "", "iso"},
	{"netboot.xyz.iso", "netbootxyz", "", "iso"},
	{"nhos-1.2.8.img", "nicehash-os", "1.2.8", "disk"},
	{"Parrot-home-6.2_amd64.iso", "parrot-home-amd64", "6.2", "iso"},
	{"pfSense-CE-2.6.0-RELEASE-amd64.iso", "pfsense-ce-amd64", "2.6.0", "iso"},
	{"pfSense-CE-2.7.1-RELEASE-amd64.iso.gz", "pfsense-ce-amd64", "2.7.1", ""},
	{"piCore.img.gz", "", "", ""},
	{"pop-os_22.04_amd64_intel_21.iso", "popos-2204-intel", "21", "iso"},
	{"pop-os_22.04_amd64_intel_36.iso", "popos-2204-intel", "36", "iso"},
	{"pop-os_22.04_amd64_nvidia_21.iso", "popos-2204-nvidia", "21", "iso"},
	{"pop-os_22.04_amd64_nvidia_36.iso", "popos-2204-nvidia", "36", "iso"},
	{"pop-os_22.04_arm64_raspi_4.img", "popos-2204-raspi", "4", "disk"},
	{"proxmox-ve_7.3-1.iso", "proxmox-ve", "7.3-1", "iso"},
	{"proxmox-ve_8.1-1.iso", "proxmox-ve", "8.1-1", "iso"},
	{"q4os-5.5-i386-instcd.r1_x32.iso", "q4os-instcd-x32", "5.5", "iso"},
	{"Qubes-R4.2.0-x86_64.iso", "qubes", "4.2.0", "iso"},
	{"Recovery.txt", "", "", ""},
	{"rescuezilla-2.4.2-64bit.jammy.iso", "rescuezilla-64bit", "2.4.2", "iso"},
	{"systemrescue-10.02-amd64.iso", "systemrescue-amd64", "10.02", "iso"},
	{"tails-amd64-5.21.img", "tails-amd64", "5.21", "disk"},
	{"TinyCore-current.iso", "tinycore-tinycore", "", "iso"},
	{"TrueNAS-13.0-U3.1.iso", "truenas-core-13", "13.0-U3.1", "iso"},
	{"TrueNAS-13.0-U6.iso", "truenas-core-13", "13.0-U6", "iso"},
	{"truenas-from proxmox.tar", "", "", ""},
	{"TrueNAS-SCALE-22.02.1.iso", "truenas-scale", "22.02.1", "iso"},
	{"TrueNAS-SCALE-22.12.0.iso", "truenas-scale", "22.12.0", "iso"},
	{"TrueNAS-SCALE-23.10.0.1.iso", "truenas-scale", "23.10.0.1", "iso"},
	{"TrueNAS-SCALE-24.10.0-HexOS.iso", "hexos-installer", "24.10.0", "iso"},
	{"ubuntu-20.04.3-desktop-amd64.iso", "ubuntu-desktop-lts", "20.04.3", "iso"},
	{"ubuntu-22.04-desktop-amd64.iso", "ubuntu-desktop-lts", "22.04", "iso"},
	{"ubuntu-22.04.3-desktop-amd64.iso", "ubuntu-desktop-lts", "22.04.3", "iso"},
	{"ubuntu-22.04.3-live-server-amd64.iso", "ubuntu-server-lts", "22.04.3", "iso"},
	{"ubuntu-core-20-amd64+intel-iot.img", "ubuntu-core-amd64-intel-iot", "20", "disk"},
	{"ubuntu-core-20-amd64.img", "ubuntu-core-amd64", "20", "disk"},
	{"ubuntu-core-20-arm64+raspi.img", "ubuntu-core-arm64-raspi", "20", "disk"},
	{"ubuntu-core-20-armhf+raspi.img", "ubuntu-core-armhf-raspi", "20", "disk"},
	{"Win11_23H2_English_x64v2.iso", "windows-11-x64", "23H2", "iso"},
	{"Win11_English_x64.iso", "windows-11-x64", "", "iso"},
	{"Windows.iso", "", "", "iso"},
	{"Windows10.iso", "windows-10", "", "iso"},
	{"Windows11.iso", "windows-11-x64", "", "iso"},
	{"Windows11_InsiderPreview_Client_x64_en-us_26016.iso", "windows-11-insider-x64", "26016", "iso"},
}
