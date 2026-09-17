package catalog

import "testing"

// The built-in catalog ships inside the binary, so it must always be valid,
// and it must recognize every image on the sample drive in CLAUDE.md.
func TestDefaultCatalog(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		filename    string
		wantID      string // empty: no entry matches
		wantVersion string
	}{
		{"Atlas_v0.5.2.iso", "atlasos", "0.5.2"},
		{"Core-current.iso", "tinycore-core", ""},
		{"CorePlus-current.iso", "tinycore-coreplus", ""},
		{"TinyCore-current.iso", "tinycore-tinycore", ""},
		{"HBCD_PE_x64.iso", "hirens-bootcd-pe", ""},
		{"FydeOS_for_PC_iris_v22.0-io-stable.iso", "fydeos-pc-iris", "22.0"},
		{"FydeOS_for_PC_iris_v22.0-SP1-io.bin", "fydeos-pc-iris", "22.0-SP1"},
		{"bazzite-deck-gnome-stable-live-amd64.iso", "bazzite-deck-gnome", ""},
		{"cachyos-desktop-linux-260308.iso", "cachyos-desktop", "260308"},
		{"Win11_English_x64.iso", "windows-11-x64", ""},
		{"Win11_23H2_English_x64v2.iso", "windows-11-x64", "23H2"},
		{"Windows10.iso", "windows-10", ""},
		{"Windows.iso", "", ""}, // Media Creation Tool's default name: could be 10 or 11
		{"MX-21.3_x64.iso", "mx-linux-xfce-x64", "21.3"},
		{"MX-23.3_x32.iso", "mx-linux-xfce-x32", "23.3"},
		{"manjaro-xfce-23.0.4-231015-linux65.iso", "manjaro-xfce", "23.0.4"},
		{"netboot.xyz.iso", "netbootxyz", ""},
		{"netboot.xyz-multiarch.iso", "netbootxyz-multiarch", ""},
		{"clonezilla-live-20231102-mantic-amd64.iso", "clonezilla-alternative", "20231102"},
		{"CentOS-7-i386-Minimal-2009.iso", "centos-7-i386-minimal", "2009"},
		{"q4os-5.5-i386-instcd.r1_x32.iso", "q4os-instcd-x32", "5.5"},
		{"batocera-5.25-x86-20200310.img", "batocera-x86", "5.25"},
		{"pop-os_22.04_amd64_intel_56.iso", "popos-2204-intel", "56"},
		{"TrueNAS-SCALE-24.10.2.2-HexOS.iso", "hexos-installer", "24.10.2.2"},
		{"linuxmint-22.3-cinnamon-64bit.iso", "linuxmint-cinnamon", "22.3"},
		{"notes.txt", "", ""},

		// Other tracks must not be mistaken for these.
		{"linuxmint-22.3-xfce-64bit.iso", "", ""},
		{"MX-25.2_Xfce_ahs_x64.iso", "", ""},
		{"manjaro-xfce-26.1.0-minimal-260812-linux71.iso", "", ""},
		{"netboot.xyz-arm64.iso", "", ""},
		{"CorePure64-current.iso", "", ""},
		{"CentOS-7-i386-Everything-2009.iso", "", ""},
	}
	for _, tt := range tests {
		matches := c.Match(tt.filename)
		if tt.wantID == "" {
			if len(matches) != 0 {
				t.Errorf("%s: matched %q, want no match", tt.filename, matches[0].Entry.ID)
			}
			continue
		}
		if len(matches) != 1 {
			t.Errorf("%s: %d matches, want exactly %q", tt.filename, len(matches), tt.wantID)
			continue
		}
		if got := matches[0]; got.Entry.ID != tt.wantID || got.Version != tt.wantVersion {
			t.Errorf("%s: matched %q version %q, want %q version %q",
				tt.filename, got.Entry.ID, got.Version, tt.wantID, tt.wantVersion)
		}
	}
}
