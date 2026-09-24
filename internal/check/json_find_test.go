package check

import (
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
)

// The note says the newest version where it says {version}, and something a
// person reads as a placeholder when the check hasn't found one.
func TestTheFindNoteNamesTheNewestVersion(t *testing.T) {
	e := &catalog.Entry{ID: "mx", Name: "MX Linux", Find: "The 32-bit ISO is MX-{version}_386.iso."}
	for _, c := range []struct{ latest, want string }{
		{"23.6", "The 32-bit ISO is MX-23.6_386.iso."},
		{"", "The 32-bit ISO is MX-VERSION_386.iso."},
	} {
		r := &Report{Items: []Item{{Path: "MX-23.3_x32.iso", Entry: e, Latest: c.latest, Status: UpdateAvailable}}}
		if got := r.JSON().Items[0].Find; got != c.want {
			t.Errorf("latest %q: find is %q, want %q", c.latest, got, c.want)
		}
	}
}
