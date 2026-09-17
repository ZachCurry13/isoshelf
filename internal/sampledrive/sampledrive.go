// Package sampledrive lists the real filenames from the maintainer's Ventoy
// drive (see CLAUDE.md), for use as test fixtures.
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
