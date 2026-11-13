package source

import (
	"context"
	"errors"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/version"
)

// TestVersionsRecorded lists the older versions of real entries, one of each
// kind of source, against the recorded responses: more than one, newest
// first, and the first is the one Latest reports.
func TestVersionsRecorded(t *testing.T) {
	client := recordedClient()
	for _, id := range []string{"mx-linux-xfce-x64", "netbootxyz", "cachyos-desktop"} {
		e := entry(t, id)
		all, err := Versions(context.Background(), client, e)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(all) < 2 {
			t.Errorf("%s: %d versions, want older ones too", id, len(all))
		}
		latest, err := Latest(context.Background(), client, e)
		if err != nil || latest.Version != all[0].Version {
			t.Errorf("%s: Latest is %v (%v), the list starts with %s", id, latest, err, all[0].Version)
		}
		if e.Source.Type == catalog.SourceListing {
			for i := 1; i < len(all); i++ {
				if version.Compare(all[i-1].Version, all[i].Version) <= 0 {
					t.Errorf("%s: %s comes before %s", id, all[i-1].Version, all[i].Version)
				}
			}
		}
		for _, r := range all {
			if e.Source.Type == catalog.SourceEndOfLife && r.Cycle == "" {
				t.Errorf("%s: %s has no cycle, so its file can't be found", id, r.Version)
			}
		}
	}
}

func TestVersionsManual(t *testing.T) {
	e := &catalog.Entry{Source: catalog.Source{Type: catalog.SourceManual}}
	if _, err := Versions(context.Background(), recordedClient(), e); !errors.Is(err, ErrManual) {
		t.Errorf("a manual entry gave %v, want ErrManual", err)
	}
}
