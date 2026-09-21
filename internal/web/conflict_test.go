package web

import (
	"context"
	"net/http"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/update"
)

// A name clash is a question, not a failure: the page has to be able to tell
// the difference so it can offer the answers instead of "Try again".
func TestANameClashAsksInsteadOfFailing(t *testing.T) {
	dirs := testDirs(t)
	drive := sampleDrive(t)
	s := newServer(t, dirs, drive)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	// Stand in for the download itself: what matters here is how the server
	// records the one error only the user can answer.
	s.runJob = func(ctx context.Context, j *job) (string, error) {
		return "", update.ErrSameName
	}
	rec := request(t, s, http.MethodPost, "/api/update",
		map[string]string{"entry": "popos-2204-intel", "removal": "keep"})
	if rec.Code != http.StatusOK && rec.Code != http.StatusAccepted {
		t.Fatalf("queue: %d %s", rec.Code, rec.Body)
	}
	st := waitIdle(t, s)
	if len(st.Downloads.Finished) == 0 {
		t.Fatal("nothing finished")
	}
	got := st.Downloads.Finished[0]
	if !got.Conflict {
		t.Errorf("a name clash wasn't marked as one: %+v", got)
	}
	if got.Message == "" {
		t.Error("the clash says nothing about what to do")
	}
}
