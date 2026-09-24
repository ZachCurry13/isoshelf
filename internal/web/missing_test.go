package web

import (
	"net/http"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// "Stop expecting it" takes a missing image off the list at once, not at the
// next scan, unstars it, and leaves the rest of the list as it was.
func TestStopExpectingTakesTheRowOffAtOnce(t *testing.T) {
	dirs, target := testDirs(t), t.TempDir()
	st := state.New(scan.Folder)
	at := time.Now().Add(-time.Hour)
	st.History = []state.ScanRecord{
		{Time: at, Entries: []string{"netbootxyz"}},
		{Time: at.Add(time.Minute), Entries: []string{"netbootxyz"}},
	}
	st.SetTrack("netbootxyz", state.Track{Starred: true})
	if err := st.Save(target); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	if !missing(waitIdle(t, s), "netbootxyz") {
		t.Fatal("netboot.xyz isn't listed as missing to begin with")
	}

	rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "netbootxyz", "not_expected": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("stop expecting: %d %s", rec.Code, rec.Body)
	}
	got := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil))
	if missing(got, "netbootxyz") {
		t.Error("netboot.xyz is still listed as missing")
	}
	if tr := got.Tracks["netbootxyz"]; tr.Starred || !tr.NotExpected {
		t.Errorf("the track is %+v, want unstarred and not expected", tr)
	}
}

func missing(st stateJSON, entry string) bool {
	if st.Report == nil {
		return false
	}
	for _, it := range st.Report.Items {
		if it.Status == string(check.Missing) && it.Entry == entry {
			return true
		}
	}
	return false
}
