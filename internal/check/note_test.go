package check

import (
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// A file that won't boot is told what would fix it, in words a person would
// use. The note used to promise a "Make bootable" button that doesn't exist,
// and to show the catalog's own word for the fix ("rename:.img").
func TestWontBootNoteSaysWhatToDo(t *testing.T) {
	for _, c := range []struct {
		fixup   string
		profile scan.Profile
		want    string
	}{
		{"rename:.img", scan.Ventoy, "Rename it to end in .img and it will boot."},
		{"extract", scan.Ventoy, "unpack it"},
		{"", scan.Proxmox, "Proxmox only lists .iso, .img files."},
	} {
		note := notBootableNote(&catalog.Entry{Fixup: c.fixup}, c.profile)
		if !strings.Contains(note, c.want) {
			t.Errorf("%q in %s: %q, want it to say %q", c.fixup, c.profile, note, c.want)
		}
		for _, never := range []string{"Make bootable", "rename:", "ventoy", "proxmox"} {
			if strings.Contains(note, never) {
				t.Errorf("%q in %s: %q says %q", c.fixup, c.profile, note, never)
			}
		}
	}
}
