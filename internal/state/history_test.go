package state

import (
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

var historyNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func kinds(events []Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Kind+" "+e.Path)
	}
	return out
}

// A new file arriving as an older one of the same image leaves is one event,
// "updated", saying what it replaced; not a download and a separate removal.
func TestAnUpdateIsOneEventWhichEverWayTheOldFileLeft(t *testing.T) {
	st := New(scan.Ventoy)
	// The new file has its version in its name, so the old one was moved aside.
	st.Files["tails-7.13.iso"] = FileRecord{Entry: "tails", Version: "7.13", Size: 200,
		Origin: Origin{How: OriginDownload, At: historyNow.Add(-time.Hour)}}
	st.Past = []ArchiveEntry{{Path: "tails-7.12.iso", Entry: "tails", Version: "7.12", Size: 190,
		Gone: GoneMovedAside, GoneAt: historyNow.Add(-time.Hour + time.Minute)}}
	// This one kept its name, so the new file says what it replaced and the
	// note about the old one is the same moment.
	st.Files["mint.iso"] = FileRecord{Entry: "mint", Version: "22", Size: 300,
		Origin: Origin{How: OriginDownload, At: historyNow.Add(-2 * time.Hour), Replaced: "21"}}
	st.Past = append(st.Past, ArchiveEntry{Path: "mint.iso", Entry: "mint", Version: "21", Size: 290,
		Gone: GoneReplaced, GoneAt: historyNow.Add(-2 * time.Hour)})

	events := st.Events(time.Time{})
	if len(events) != 2 {
		t.Fatalf("%d events %v, want the two updates and nothing else", len(events), kinds(events))
	}
	for _, e := range events {
		if e.Kind != EventUpdated || e.From == "" || !e.Here {
			t.Errorf("%+v: want an update from an older version, with the new file here", e)
		}
	}
	if events[0].Path != "tails-7.13.iso" || events[0].From != "7.12" || events[1].From != "21" {
		t.Errorf("got %v from %q and %q, want tails first, from 7.12, then mint from 21", kinds(events), events[0].From, events[1].From)
	}
}

// Things that are not an update stay what they are.
func TestEventsThatAreNotUpdatesStayWhatTheyAre(t *testing.T) {
	st := New(scan.Ventoy)
	// Downloaded, and a different image's old file leaving at the same moment.
	st.Files["a.iso"] = FileRecord{Entry: "a", Size: 1, Origin: Origin{How: OriginDownload, At: historyNow.Add(-time.Hour)}}
	st.Past = append(st.Past, ArchiveEntry{Path: "b-old.iso", Entry: "b", Size: 2, Gone: GoneRemoved, GoneAt: historyNow.Add(-time.Hour)})
	// An older file of the same image gone a day later: a removal, not part of the update.
	st.Files["c.iso"] = FileRecord{Entry: "c", Size: 3, Origin: Origin{How: OriginCopy, At: historyNow.Add(-48 * time.Hour)}}
	st.Past = append(st.Past, ArchiveEntry{Path: "c-old.iso", Entry: "c", Size: 4, Gone: GoneMovedAside, GoneAt: historyNow.Add(-24 * time.Hour)})
	// Added by hand is never an update, and a file that vanished between scans
	// says nothing of when, so it is never paired with anything.
	st.Files["d.iso"] = FileRecord{Entry: "d", Size: 5, Origin: Origin{How: OriginUpload, At: historyNow.Add(-3 * time.Hour)}}
	st.Past = append(st.Past, ArchiveEntry{Path: "d-old.iso", Entry: "d", Size: 6, Gone: GoneRemoved, GoneAt: historyNow.Add(-3 * time.Hour)})
	st.Files["e.iso"] = FileRecord{Entry: "e", Size: 7, Origin: Origin{How: OriginDownload, At: historyNow.Add(-5 * time.Hour)}}
	st.Past = append(st.Past, ArchiveEntry{Path: "e-old.iso", Entry: "e", Size: 8, Gone: GoneVanished, GoneAt: historyNow.Add(-5 * time.Hour)})
	// Found in the folder, never arrived through isoshelf: nothing to say.
	st.Files["found.iso"] = FileRecord{Size: 9}

	want := map[string]string{
		"a.iso": EventDownloaded, "b-old.iso": EventDeleted,
		"c.iso": EventCopied, "c-old.iso": EventArchived,
		"d.iso": EventAdded, "d-old.iso": EventDeleted,
		"e.iso": EventDownloaded, "e-old.iso": EventVanished,
	}
	events := st.Events(time.Time{})
	if len(events) != len(want) {
		t.Fatalf("%d events %v, want %d", len(events), kinds(events), len(want))
	}
	for _, e := range events {
		if want[e.Path] != e.Kind {
			t.Errorf("%s is %q, want %q", e.Path, e.Kind, want[e.Path])
		}
	}
}

// Newest first, only from the date asked for, and a download from before
// origins were kept still counts as one.
func TestEventsAreNewestFirstAndOnlyFromTheDateAsked(t *testing.T) {
	st := New(scan.Ventoy)
	st.Files["new.iso"] = FileRecord{Size: 1, PlacedAt: historyNow.Add(-24 * time.Hour)} // before v0.8.0
	st.Files["mid.iso"] = FileRecord{Size: 1, Origin: Origin{How: OriginUpload, At: historyNow.Add(-10 * 24 * time.Hour)}}
	st.Past = []ArchiveEntry{{Path: "old.iso", Size: 1, Gone: GoneRemoved, GoneAt: historyNow.Add(-100 * 24 * time.Hour)}}

	all := st.Events(time.Time{})
	if got := kinds(all); len(got) != 3 || got[0] != "downloaded new.iso" || got[1] != "added mid.iso" || got[2] != "deleted old.iso" {
		t.Errorf("everything: %v", got)
	}
	if got := kinds(st.Events(historyNow.AddDate(0, 0, -30))); len(got) != 2 {
		t.Errorf("last 30 days: %v, want the two recent ones", got)
	}
}
