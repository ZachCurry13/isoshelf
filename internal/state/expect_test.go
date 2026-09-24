package state

import (
	"slices"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// "Stop expecting it" (#55) takes a missing image off the missing list, star
// and all, until the image turns up in the folder again - and then it is
// expected as usual, because somebody put it back.
func TestAnImageStopsBeingExpectedUntilItIsBack(t *testing.T) {
	cat := defaultCatalog(t)
	s := New(scan.Ventoy)
	mint := scanned(cat, "linuxmint-22.1-cinnamon-64bit.iso", 100, t0)
	if len(mint.Matches) != 1 {
		t.Fatalf("the test file matches %d entries, want 1", len(mint.Matches))
	}
	id := mint.Matches[0].Entry.ID
	at := t0.Add(time.Hour)
	s.RecordScan(&scan.Result{Files: []scan.File{mint}}, at)
	s.RecordScan(&scan.Result{Files: []scan.File{mint}}, at.Add(time.Minute))
	s.RecordScan(&scan.Result{}, at.Add(2*time.Minute))
	s.SetTrack("bazzite", Track{Starred: true})
	if got := s.Missing(); !slices.Contains(got, id) || !slices.Contains(got, "bazzite") {
		t.Fatalf("missing = %v, want %s and bazzite", got, id)
	}

	s.SetTrack(id, Track{NotExpected: true})
	s.SetTrack("bazzite", Track{Starred: true, NotExpected: true})
	if got := s.Missing(); len(got) != 0 {
		t.Errorf("missing = %v after stopping expecting both, want nothing", got)
	}
	s.RecordScan(&scan.Result{}, at.Add(3*time.Minute))
	if got := s.UsualSet(); len(got) != 0 {
		t.Errorf("usual set = %v a scan later, want nothing: not expected lasts until the image is back", got)
	}

	s.RecordScan(&scan.Result{Files: []scan.File{mint}}, at.Add(4*time.Minute))
	if s.Track(id).NotExpected {
		t.Error("the image is back in the folder but still not expected")
	}
	if !slices.Contains(s.UsualSet(), id) {
		t.Errorf("usual set = %v once the image is back, want %s in it", s.UsualSet(), id)
	}
	if !s.Track("bazzite").NotExpected {
		t.Error("bazzite was never found, so it should still not be expected")
	}
}
