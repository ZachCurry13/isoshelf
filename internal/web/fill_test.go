package web

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/drivelist"
	"github.com/ZachCurry13/isoshelf/internal/space"
)

// A drive's list (#11, #12): saved from one folder, then used to fill an
// empty one - from the copy this computer remembers, and from the saved
// file - through the ordinary downloads, with its stars carried over and
// what can't be fetched said rather than skipped quietly.
func TestFillAFolderFromAList(t *testing.T) {
	dirs, drive := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, drive)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)
	request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "netbootxyz", "starred": true})

	rec := request(t, s, http.MethodGet, "/api/list", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("list: %d %v", rec.Code, rec.Header())
	}
	saved := rec.Body.String()
	l, err := drivelist.Decode([]byte(saved))
	if err != nil || len(l.Images) < 10 {
		t.Fatalf("the saved list: %d images, %v", len(l.Images), err)
	}

	var mu sync.Mutex
	var fetched []string
	s.runJob = func(ctx context.Context, j *job) (string, error) {
		mu.Lock()
		fetched = append(fetched, j.entry)
		mu.Unlock()
		return "", nil
	}
	oldID := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).Recent[0].ID
	empty := t.TempDir()
	if rec := request(t, s, http.MethodPost, "/api/target", map[string]string{"path": empty, "profile": "ventoy"}); rec.Code != http.StatusOK {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body)
	}
	waitIdle(t, s)

	for _, from := range []map[string]any{{"folder": oldID}, {"text": saved}} {
		p := decode[fillPlan](t, request(t, s, http.MethodPost, "/api/fill/plan", from))
		if len(p.Download) == 0 || p.Bytes == 0 {
			t.Fatalf("%v: plan %+v", from, p)
		}
		for _, c := range p.Cannot {
			if c.Why == "" {
				t.Errorf("%s can't be fetched and the plan doesn't say why", c.Name)
			}
		}
	}

	ids := []string{}
	p := decode[fillPlan](t, request(t, s, http.MethodPost, "/api/fill/plan", map[string]any{"text": saved}))
	slices.SortFunc(p.Download, func(a, b fillItem) int { return cmp.Compare(a.Size, b.Size) })
	for _, d := range p.Download[:3] {
		ids = append(ids, d.ID)
	}
	// Room first: everything on the list onto a folder with 1 GB free is
	// refused before anything starts.
	s.mu.Lock()
	s.room = space.Usage{Free: 1 << 30, Total: 100 << 30}
	s.mu.Unlock()
	all := []string{}
	for _, d := range p.Download {
		all = append(all, d.ID)
	}
	if rec := request(t, s, http.MethodPost, "/api/fill", map[string]any{"text": saved, "ids": all}); rec.Code != http.StatusConflict {
		t.Errorf("a fill that can't fit answered %d", rec.Code)
	}
	s.mu.Lock()
	s.room = space.Usage{Free: 1 << 40, Total: 2 << 40}
	s.mu.Unlock()
	if rec := request(t, s, http.MethodPost, "/api/fill", map[string]any{"text": saved, "ids": ids}); rec.Code != http.StatusAccepted {
		t.Fatalf("fill: %d %s", rec.Code, rec.Body)
	}
	waitIdle(t, s)
	mu.Lock()
	defer mu.Unlock()
	if len(fetched) != 3 {
		t.Errorf("downloaded %v, want the 3 ticked", fetched)
	}
	if !decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).Tracks["netbootxyz"].Starred {
		t.Error("the star on the old folder didn't come to the new one")
	}
	if rec := request(t, s, http.MethodPost, "/api/fill/plan", map[string]any{"text": "not a list ["}); rec.Code != http.StatusBadRequest {
		t.Errorf("a file that isn't a list answered %d", rec.Code)
	}
}
