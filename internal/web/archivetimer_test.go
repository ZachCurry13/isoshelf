package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// archiveWith puts files in .isoshelf/removed and records when each of them
// left, which is what the timer judges them by.
func archiveWith(t *testing.T, target string, files map[string]time.Time) *state.State {
	t.Helper()
	dir := filepath.Join(target, state.DirName, update.RemovedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Folder)
	for name, goneAt := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("an image"), 0o644); err != nil {
			t.Fatal(err)
		}
		st.Past = append(st.Past, state.ArchiveEntry{
			Path: name, Size: 8, Gone: state.GoneMovedAside, GoneAt: goneAt,
		})
	}
	return st
}

// The whole point of the timer is that each file is judged by its own age.
// A sweep that clears a file from three months ago must leave this morning's
// alone, because the archive is the undo for every removal isoshelf makes.
func TestTheArchiveTimerJudgesEachFileByItsOwnAge(t *testing.T) {
	target := t.TempDir()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	st := archiveWith(t, target, map[string]time.Time{
		"ancient.iso": now.Add(-100 * 24 * time.Hour),
		"older.iso":   now.Add(-31 * 24 * time.Hour),
		"recent.iso":  now.Add(-2 * 24 * time.Hour),
		"today.iso":   now.Add(-time.Hour),
	})

	names, bytes := staleArchived(target, st, 30, now)
	if len(names) != 2 {
		t.Fatalf("30 days took %v, want the two older than 30 days", names)
	}
	for _, name := range names {
		if name == "recent.iso" || name == "today.iso" {
			t.Errorf("the timer would delete %s, which is younger than 30 days", name)
		}
	}
	if bytes != 16 {
		t.Errorf("the space it would free is %d, want 16", bytes)
	}
}

// Off is off. A number nobody chose must never start deleting, which is what
// an upgrade that defaulted this on would do.
func TestTheArchiveTimerDoesNothingUntilANumberIsChosen(t *testing.T) {
	target := t.TempDir()
	now := time.Now()
	st := archiveWith(t, target, map[string]time.Time{
		"ancient.iso": now.Add(-1000 * 24 * time.Hour),
	})

	if names, _ := staleArchived(target, st, 0, now); len(names) != 0 {
		t.Errorf("with no number chosen the timer would delete %v", names)
	}
	for _, days := range []int{1, 5, 45, 365, -30} {
		if got := cleanArchiveAfter(days); got != 0 {
			t.Errorf("cleanArchiveAfter(%d) = %d, want 0: only 7, 30 and 90 are offered", days, got)
		}
	}
	for _, days := range []int{7, 30, 90} {
		if got := cleanArchiveAfter(days); got != days {
			t.Errorf("cleanArchiveAfter(%d) = %d, want it kept", days, got)
		}
	}
}

// A file isoshelf has no record for has no age it can judge, so it stays -
// however long it has been sitting there. Something in .isoshelf/removed that
// isoshelf didn't put there is not isoshelf's to delete on a timer.
func TestTheArchiveTimerLeavesFilesItHasNoRecordFor(t *testing.T) {
	target := t.TempDir()
	now := time.Now()
	st := archiveWith(t, target, map[string]time.Time{
		"known.iso": now.Add(-100 * 24 * time.Hour),
	})
	dir := filepath.Join(target, state.DirName, update.RemovedDir)
	if err := os.WriteFile(filepath.Join(dir, "a-stranger.iso"), []byte("not ours"), 0o644); err != nil {
		t.Fatal(err)
	}

	names, _ := staleArchived(target, st, 7, now)
	for _, name := range names {
		if name == "a-stranger.iso" {
			t.Fatal("the timer would delete a file isoshelf has no archive record for")
		}
	}
	if len(names) != 1 || names[0] != "known.iso" {
		t.Errorf("the timer took %v, want just the one it has a record for", names)
	}

	// And with no records at all, nothing is old enough for anything.
	if got, _ := staleArchived(target, nil, 7, now); len(got) != 0 {
		t.Errorf("with no records the timer would delete %v", got)
	}
}

// RemoveArchived is the timer's hands: it takes the names it was given and
// nothing else, unlike EmptyRemoved which clears the lot on a button press.
func TestRemovingArchivedFilesTakesOnlyWhatItWasGiven(t *testing.T) {
	target := t.TempDir()
	now := time.Now()
	archiveWith(t, target, map[string]time.Time{
		"go.iso":   now.Add(-100 * 24 * time.Hour),
		"stay.iso": now.Add(-100 * 24 * time.Hour),
	})

	deleted, err := update.RemoveArchived(target, []string{"go.iso"})
	if err != nil || deleted != 1 {
		t.Fatalf("deleted %d, err %v; want 1 and no error", deleted, err)
	}
	dir := filepath.Join(target, state.DirName, update.RemovedDir)
	if _, err := os.Stat(filepath.Join(dir, "go.iso")); err == nil {
		t.Error("the file it was asked to delete is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "stay.iso")); err != nil {
		t.Error("it deleted a file it wasn't asked about")
	}

	// A name that isn't a plain filename is refused rather than resolved.
	if _, err := update.RemoveArchived(target, []string{"../stay.iso"}); err == nil {
		t.Error("a name with a path in it was accepted")
	}
	if _, err := os.Stat(filepath.Join(target, state.DirName, update.RemovedDir, "stay.iso")); err != nil {
		t.Error("a name with a path in it reached outside the archive")
	}
}

// The sweep itself, end to end: it takes the stale file off the drive, leaves
// the fresh one, and says afterwards what it did - because this happened
// while nobody was watching.
func TestTheArchiveSweepDeletesOnlyWhatHasWaitedLongEnough(t *testing.T) {
	dirs, target := testDirs(t), t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "here.iso"), []byte("an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	now := s.cfg.Now()
	dir := filepath.Join(target, state.DirName, update.RemovedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	for name, goneAt := range map[string]time.Time{
		"stale.iso": now.Add(-100 * 24 * time.Hour),
		"fresh.iso": now.Add(-2 * 24 * time.Hour),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("an image"), 0o644); err != nil {
			t.Fatal(err)
		}
		s.st.Past = append(s.st.Past, state.ArchiveEntry{
			Path: name, Size: 8, Gone: state.GoneMovedAside, GoneAt: goneAt,
		})
	}
	s.mu.Unlock()

	// Off: the sweep must do nothing at all.
	s.emptyArchiveIfDue()
	if _, err := os.Stat(filepath.Join(dir, "stale.iso")); err != nil {
		t.Fatal("the sweep deleted a file with no number chosen")
	}

	if rec := request(t, s, http.MethodPost, "/api/settings", map[string]any{"archive_after": 30}); rec.Code != http.StatusOK {
		t.Fatalf("choosing 30 days: %d %s", rec.Code, rec.Body)
	}
	s.emptyArchiveIfDue()

	if _, err := os.Stat(filepath.Join(dir, "stale.iso")); err == nil {
		t.Error("the file that waited 100 days is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "fresh.iso")); err != nil {
		t.Error("the sweep took a file that had waited two days")
	}
	s.mu.Lock()
	note := s.autoNote
	s.mu.Unlock()
	if !strings.Contains(note, "deleted") {
		t.Errorf("the page is told %q; it should say what was deleted while nobody watched", note)
	}
}
