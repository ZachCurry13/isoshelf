package lastcheck

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

func at(day int) time.Time {
	return time.Date(2026, 9, day, 12, 0, 0, 0, time.UTC)
}

// An answer from this morning is used as it stands; yesterday's is not.
func TestAnswersAreUsedForADay(t *testing.T) {
	a := Load(t.TempDir())
	a.Now = func() time.Time { return at(10) }
	a.Remember("ubuntu", &source.Release{Version: "24.04.1"}, nil)

	a.Now = func() time.Time { return at(10).Add(6 * time.Hour) }
	if rel, _, ok := a.Recall("ubuntu"); !ok || rel.Version != "24.04.1" {
		t.Errorf("six hours later: %v %v", rel, ok)
	}
	a.Now = func() time.Time { return at(12) }
	if _, _, ok := a.Recall("ubuntu"); ok {
		t.Error("an answer from two days ago was used")
	}
	if _, _, ok := a.Recall("nothing-known"); ok {
		t.Error("an entry nobody asked about came back with an answer")
	}
}

// What one run learns, the next run starts with: that is the whole point.
func TestAnswersSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	a := Load(dir)
	a.Now = func() time.Time { return at(10) }
	a.Remember("debian", &source.Release{Version: "12.7.0", URL: "https://example.test/12.7.0"},
		&resolve.Artifact{
			Filename: "debian-12.7.0-amd64-netinst.iso",
			Checksum: &verify.Checksum{Algorithm: verify.SHA256, Hex: "abc123"},
		})
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	next := Load(dir)
	next.Now = func() time.Time { return at(10).Add(time.Hour) }
	rel, art, ok := next.Recall("debian")
	if !ok {
		t.Fatal("nothing was remembered")
	}
	if rel.Version != "12.7.0" || rel.URL != "https://example.test/12.7.0" {
		t.Errorf("release came back as %+v", rel)
	}
	if art == nil || art.Filename != "debian-12.7.0-amd64-netinst.iso" || art.Checksum.Hex != "abc123" {
		t.Errorf("file came back as %+v", art)
	}
	if got := next.Asked("debian"); !got.Equal(at(10)) {
		t.Errorf("answer is dated %v, want %v", got, at(10))
	}
}

// A project that was down must be asked again, not remembered as bad news.
func TestAFailureIsNotRemembered(t *testing.T) {
	a := Load(t.TempDir())
	a.Remember("fedora", nil, nil)
	if _, _, ok := a.Recall("fedora"); ok {
		t.Error("a failure was remembered")
	}
}

// Refresh asks every project again, and keeps what comes back.
func TestAskingNeverReuses(t *testing.T) {
	a := Load(t.TempDir())
	a.Now = func() time.Time { return at(10) }
	a.Remember("mint", &source.Release{Version: "22.3"}, nil)

	asking := a.Asking()
	if _, _, ok := asking.Recall("mint"); ok {
		t.Error("Refresh reused an answer instead of asking again")
	}
	asking.Remember("mint", &source.Release{Version: "22.4"}, nil)
	if rel, _, ok := a.Recall("mint"); !ok || rel.Version != "22.4" {
		t.Errorf("the new answer wasn't kept: %v %v", rel, ok)
	}
}

// Answers nobody has wanted for a month stop being kept, so the file can't
// grow by every image that ever left the catalog.
func TestOldAnswersAreDropped(t *testing.T) {
	dir := t.TempDir()
	a := Load(dir)
	a.Now = func() time.Time { return at(1) }
	a.Remember("long-gone", &source.Release{Version: "1"}, nil)
	a.Now = func() time.Time { return at(1).Add(60 * 24 * time.Hour) }
	a.Remember("still-here", &source.Release{Version: "2"}, nil)
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	next := Load(dir)
	if _, ok := next.Entries["long-gone"]; ok {
		t.Error("an answer from two months ago was kept")
	}
	if _, ok := next.Entries["still-here"]; !ok {
		t.Error("today's answer was dropped")
	}
}

// A damaged file is no reason to fail: these are answers isoshelf can ask
// for again.
func TestADamagedFileIsTreatedAsNoFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(filepath.Join(dir, FileName), "{not json"); err != nil {
		t.Fatal(err)
	}
	if a := Load(dir); len(a.Entries) != 0 {
		t.Errorf("a damaged file gave %d answers", len(a.Entries))
	}
}

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}
