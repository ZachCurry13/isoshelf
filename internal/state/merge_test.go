package state

import (
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// A download started from one copy of the state, and while it ran the user
// removed a file and starred an image. When the download saves, both sets of
// changes have to survive.
func TestMergeKeepsBothWriters(t *testing.T) {
	start := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	disk := New(scan.Folder)
	disk.Files["old.iso"] = FileRecord{Size: 1, Entry: "example", Version: "1"}
	disk.Files["other.iso"] = FileRecord{Size: 2, Entry: "other"}
	disk.History = []ScanRecord{{Time: start}}

	// The download's copy: the new file placed, the old one replaced.
	base := disk.Clone()
	download := base.Clone()
	download.Files["new.iso"] = FileRecord{Size: 3, Entry: "example", Version: "2"}
	download.Archived("old.iso", GoneReplaced, start.Add(time.Hour))

	// Meanwhile, on disk: other.iso archived by hand, and a star.
	disk.Archived("other.iso", GoneMovedAside, start.Add(time.Minute))
	disk.SetTrack("netbootxyz", Track{Starred: true})

	Merge(base, download, disk)

	if _, ok := disk.Files["new.iso"]; !ok {
		t.Error("the downloaded file's record was lost")
	}
	for _, gone := range []string{"old.iso", "other.iso"} {
		if _, ok := disk.Files[gone]; ok {
			t.Errorf("%s still has a record", gone)
		}
	}
	if !disk.Track("netbootxyz").Starred {
		t.Error("the star set during the download was lost")
	}
	if len(disk.Past) != 2 || disk.Past[0].Path != "old.iso" || disk.Past[1].Path != "other.iso" {
		t.Errorf("past = %+v", disk.Past)
	}
	if len(disk.History) != 1 {
		t.Errorf("history = %+v", disk.History)
	}
}

// A change the copy didn't make never undoes one made on disk, and a record
// the copy changed wins over the old one.
func TestMergeOnlyCarriesChanges(t *testing.T) {
	disk := New(scan.Folder)
	disk.Files["a.iso"] = FileRecord{Size: 1}
	base := disk.Clone()
	changed := base.Clone()
	changed.Files["a.iso"] = FileRecord{Size: 1, Entry: "ubuntu-desktop-lts", Assigned: true}

	disk.Files["b.iso"] = FileRecord{Size: 2}
	disk.Profile = scan.Ventoy

	Merge(base, changed, disk)
	if rec := disk.Files["a.iso"]; !rec.Assigned || rec.Entry != "ubuntu-desktop-lts" {
		t.Errorf("a.iso = %+v", rec)
	}
	if _, ok := disk.Files["b.iso"]; !ok {
		t.Error("a file recorded on disk was dropped")
	}
	if disk.Profile != scan.Ventoy {
		t.Errorf("profile = %s, want the one on disk", disk.Profile)
	}
}
