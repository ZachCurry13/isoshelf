package check

import (
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// A file that won't boot is told what would fix it, in words a person would
// use, never the catalog's own word for the fix ("rename:.img"). It names
// Make bootable (#3) only for the fixes that button does.
func TestWontBootNoteSaysWhatToDo(t *testing.T) {
	for _, c := range []struct {
		fixup   string
		profile scan.Profile
		want    string
	}{
		{"rename:.img", scan.Ventoy, "Make bootable renames it to end in .img"},
		{"extract", scan.Ventoy, "Make bootable unpacks it"},
		{"convert", scan.Ventoy, "converted"},
		{"", scan.Proxmox, "Proxmox only lists .iso, .img files."},
	} {
		note := notBootableNote(&catalog.Entry{Fixup: c.fixup}, c.profile)
		if !strings.Contains(note, c.want) {
			t.Errorf("%q in %s: %q, want it to say %q", c.fixup, c.profile, note, c.want)
		}
		never := []string{"rename:", "ventoy", "proxmox"}
		if c.fixup == "convert" || c.fixup == "" {
			never = append(never, "Make bootable")
		}
		for _, never := range never {
			if strings.Contains(note, never) {
				t.Errorf("%q in %s: %q says %q", c.fixup, c.profile, note, never)
			}
		}
	}
}
