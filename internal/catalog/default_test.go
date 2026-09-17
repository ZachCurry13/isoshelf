package catalog

import (
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
)

// The built-in catalog ships inside the binary, so it must always be valid,
// and it must recognize every image on the sample drive.
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
	for _, f := range sampledrive.Files {
		tests = append(tests, test{f.Name, f.Entry, f.Version})
	}
	// Other tracks must not be mistaken for the sample drive's.
	tests = append(tests,
		test{"linuxmint-22.3-xfce-64bit.iso", "", ""},
		test{"MX-25.2_Xfce_ahs_x64.iso", "", ""},
		test{"manjaro-xfce-26.1.0-minimal-260812-linux71.iso", "", ""},
		test{"netboot.xyz-arm64.iso", "", ""},
		test{"CorePure64-current.iso", "", ""},
		test{"CentOS-7-i386-Everything-2009.iso", "", ""},
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
