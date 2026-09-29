package state

import (
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// Wednesday 2026-09-30, midday: the week began on Monday the 28th.
var usageNow = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

func TestWeeksStartOnMondayNewestFirst(t *testing.T) {
	weeks := New(scan.Ventoy).Weeks(usageNow, 3)
	want := []time.Time{
		time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
	}
	if len(weeks) != len(want) {
		t.Fatalf("got %d weeks, want %d", len(weeks), len(want))
	}
	for i, w := range weeks {
		if !w.Start.Equal(want[i]) {
			t.Errorf("week %d starts %v, want %v", i, w.Start, want[i])
		}
	}
	// A Sunday belongs to the week that began six days before, not the next.
	sunday := time.Date(2026, 10, 4, 23, 0, 0, 0, time.UTC)
	if got := weekStart(sunday); !got.Equal(want[0]) {
		t.Errorf("Sunday's week starts %v, want %v", got, want[0])
	}
}

// Each file is counted once, in the week it arrived, by how it arrived; and
// each image that left is counted in the week it left, by why.
func TestWeeksCountWhatArrivedAndWhatLeft(t *testing.T) {
	st := New(scan.Ventoy)
	thisWeek := usageNow.Add(-24 * time.Hour)
	lastWeek := usageNow.AddDate(0, 0, -7)
	st.Files["new.iso"] = FileRecord{Size: 100, Origin: Origin{How: OriginDownload, At: thisWeek}}
	st.Files["copy.iso"] = FileRecord{Size: 50, Origin: Origin{How: OriginCopy, At: thisWeek}}
	st.Files["mine.iso"] = FileRecord{Size: 10, Origin: Origin{How: OriginUpload, At: lastWeek}}
	// Downloaded before v0.8.0: no Origin, but PlacedAt says it was isoshelf.
	st.Files["old.iso"] = FileRecord{Size: 7, PlacedAt: lastWeek}
	// Found in the folder: nothing says how it got there, so it isn't counted.
	st.Files["found.iso"] = FileRecord{Size: 1000}
	// Replaced this week, having been downloaded last week: both are counted.
	st.Past = []ArchiveEntry{
		{Path: "prev.iso", Size: 90, Gone: GoneReplaced, GoneAt: thisWeek,
			Arrived: Origin{How: OriginDownload, At: lastWeek}},
		{Path: "aside.iso", Size: 5, Gone: GoneMovedAside, GoneAt: thisWeek},
		{Path: "bin.iso", Size: 3, Gone: GoneRemoved, GoneAt: lastWeek},
		// Long ago: outside every week asked for.
		{Path: "ancient.iso", Size: 1, Gone: GoneVanished, GoneAt: usageNow.AddDate(-1, 0, 0)},
	}
	st.History = []ScanRecord{{Time: thisWeek}, {Time: thisWeek.Add(time.Hour)}, {Time: lastWeek}}

	weeks := st.Weeks(usageNow, 2)
	now, before := weeks[0], weeks[1]
	checks := []struct {
		name string
		got  Tally
		want Tally
	}{
		{"downloaded this week", now.Downloaded, Tally{1, 100}},
		{"copied this week", now.Copied, Tally{1, 50}},
		{"replaced this week", now.Replaced, Tally{1, 90}},
		{"archived this week", now.Archived, Tally{1, 5}},
		{"downloaded last week", before.Downloaded, Tally{2, 97}},
		{"added last week", before.Added, Tally{1, 10}},
		{"deleted last week", before.Deleted, Tally{1, 3}},
		{"vanished at all", Tally{now.Vanished.Files + before.Vanished.Files, 0}, Tally{}},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, c.got, c.want)
		}
	}
	if now.Scans != 2 || before.Scans != 1 {
		t.Errorf("scans %d and %d, want 2 and 1", now.Scans, before.Scans)
	}
}

// A time after now - a clock that was wrong when a file arrived - belongs to
// no week, rather than to this one.
func TestWeeksIgnoreTheFuture(t *testing.T) {
	st := New(scan.Ventoy)
	st.Files["later.iso"] = FileRecord{Size: 1, Origin: Origin{How: OriginDownload, At: usageNow.Add(48 * time.Hour)}}
	if got := st.Weeks(usageNow, 1)[0].Downloaded; got != (Tally{}) {
		t.Errorf("counted a download from the future: %+v", got)
	}
}

// Archiving keeps how the file arrived, so its download still counts after
// it has gone.
func TestArchivingKeepsTheArrival(t *testing.T) {
	st := New(scan.Ventoy)
	at := usageNow.Add(-time.Hour)
	st.Files["a.iso"] = FileRecord{Size: 4, Origin: Origin{How: OriginDownload, At: at}}
	st.Archived("a.iso", GoneReplaced, usageNow)
	if got := st.Past[0].Arrived; got.How != OriginDownload || !got.At.Equal(at) {
		t.Errorf("arrival after archiving: %+v", got)
	}
}
