package catalog

import (
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
)

// The built-in catalog ships inside the binary, so it must always be valid,
// and it must recognize every image on the sample drive and in the Proxmox
// folder.
func TestDefaultCatalog(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}

	type test struct {
		filename    string
		wantID      string // empty: no entry matches
		wantVersion string
	}
	var tests []test
	for _, f := range append(sampledrive.Files, sampledrive.ProxmoxFolder...) {
		tests = append(tests, test{f.Name, f.Entry, f.Version})
	}
	// Other tracks must not be mistaken for the ones in the catalog.
	tests = append(tests,
		test{"linuxmint-22.3-xfce-64bit.iso", "linuxmint-xfce", "22.3"},
		test{"lmde-7-cinnamon-64bit.iso", "lmde-cinnamon", "7"},
		test{"Fedora-Workstation-Live-44-1.7.x86_64.iso", "fedora-workstation", "44-1.7"},
		test{"archlinux-2026.09.01-x86_64.iso", "archlinux", "2026.09.01"},
		test{"clonezilla-live-3.3.3-15-amd64.iso", "clonezilla-stable", "3.3.3-15"},
		test{"archlinux-x86_64.iso", "", ""}, // the unversioned copy
		test{"xubuntu-26.04.1-minimal-amd64.iso", "", ""}, // minimal is another track
		test{"linuxmint-22.3-mate-64bit.iso", "", ""}, // Mint MATE is not in the catalog
		test{"MX-25.2_Xfce_ahs_x64.iso", "", ""},
		test{"manjaro-xfce-26.1.0-minimal-260812-linux71.iso", "", ""},
		test{"netboot.xyz-arm64.iso", "", ""},
		test{"CorePure64-current.iso", "", ""},
		test{"CentOS-7-x86_64-DVD-2009.iso", "", ""},
		test{"ubuntu-25.10-desktop-amd64.iso", "", ""}, // interim release, not LTS
		test{"proxmox-ve_9.2-1-arm64.iso", "", ""},
		test{"TrueNAS-26.0.0-BETA.3.iso", "", ""},
		test{"Qubes-R4.3.1-rc1-x86_64.iso", "", ""},
		test{"kali-linux-2026.2-installer-netinst-amd64.iso", "", ""},
		test{"debian-edu-13.7.0-amd64-netinst.iso", "", ""},
		test{"rescuezilla-2.6.1-32bit.bionic.iso", "", ""},
	)

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
