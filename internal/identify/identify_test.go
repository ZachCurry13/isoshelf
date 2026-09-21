package identify

import (
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// file builds a scanned file with a disc label, as sniff would read it.
func file(path string, size int64, label string) scan.File {
	return scan.File{
		Path: path, Size: size, Kind: sniff.ISO, Bootable: true,
		Volume: sniff.Volume{Label: label},
	}
}

// top returns the best guess, failing the test when there is none.
func top(t *testing.T, guesses []Guess) Guess {
	t.Helper()
	if len(guesses) == 0 {
		t.Fatal("no guesses")
	}
	return guesses[0]
}

// The labels here were read out of real images on a Proxmox ISO folder.
func TestSuggestFromLabel(t *testing.T) {
	cat := testCatalog(t)
	tests := []struct {
		name    string
		label   string
		want    string // entry id
		version string
	}{
		{"copy of ubuntu.iso", "Ubuntu 22.04.3 LTS amd64", "ubuntu-desktop-lts", "22.04.3"},
		{"server.iso", "Ubuntu-Server 22.04.3 LTS amd64", "ubuntu-server-lts", "22.04.3"},
		{"rescue.iso", "Rescuezilla", "rescuezilla-64bit", ""},
		{"qubes-copy.iso", "QUBES-R4-2-0-X86-64", "qubes", ""},
		{"parrot.iso", "Parrot home 6.2", "parrot-home-amd64", "6.2"},
		{"downloaded.iso", "Pop_OS 22.04 amd64 Nvidia", "popos-2204-nvidia", "22.04"},
		// A label can be silent: Proxmox calls every one of its discs "PVE",
		// so this one is recognized by its name instead.
		{"proxmox-copy.iso", "PVE", "proxmox-ve", ""},
	}
	for _, tt := range tests {
		res := &scan.Result{Files: []scan.File{file(tt.name, 1<<30, tt.label)}}
		got := top(t, Suggest(res, state.New(scan.Proxmox), cat, tt.name))
		if got.Entry.ID != tt.want {
			t.Errorf("%s (%q): guessed %s, want %s (reason: %s, score %d)",
				tt.name, tt.label, got.Entry.ID, tt.want, got.Reason, got.Score)
			continue
		}
		if got.Version != tt.version {
			t.Errorf("%s: version %q, want %q", tt.name, got.Version, tt.version)
		}
	}
}

// Some discs genuinely can't be told apart: CentOS 7 labels its Minimal and
// Everything images exactly alike. Then both are offered, and the user picks.
func TestSuggestOffersBothWhenTheDiscIsAmbiguous(t *testing.T) {
	cat := testCatalog(t)
	res := &scan.Result{Files: []scan.File{file("old-disc.iso", 1<<30, "CentOS 7 x86_64")}}
	var offered []string
	for _, g := range Suggest(res, state.New(scan.Proxmox), cat, "old-disc.iso") {
		offered = append(offered, g.Entry.ID)
	}
	for _, want := range []string{"centos-7-x64-minimal", "centos-7-x64-everything"} {
		if !hasString(offered, want) {
			t.Errorf("%s was not offered; got %v", want, offered)
		}
	}
	for _, id := range offered {
		if id == "centos-7-i386-minimal" || id == "centos-7-i386-everything" {
			t.Errorf("offered the 32-bit %s for a 64-bit disc", id)
		}
	}
}

// A 32-bit image must never be suggested as the 64-bit track of the same
// product, whatever the names look like.
func TestSuggestKeepsArchitectureApart(t *testing.T) {
	cat := testCatalog(t)
	res := &scan.Result{Files: []scan.File{file("centos-copy.iso", 1<<30, "CentOS 7 i386")}}
	for _, g := range Suggest(res, state.New(scan.Proxmox), cat, "centos-copy.iso") {
		if e := g.Entry; e.Arch == "x86_64" {
			t.Errorf("suggested 64-bit %s for a 32-bit disc (score %d)", e.ID, g.Score)
		}
	}
}

// Windows.iso is the name the Media Creation Tool gives every image it makes,
// and its label says nothing either. What gives it away is the copy of the
// same image sitting next to it.
func TestSuggestFindsTheTwin(t *testing.T) {
	cat := testCatalog(t)
	built := time.Date(2021, 9, 20, 20, 54, 49, 0, time.UTC)
	unknown := file("Windows.iso", 4556128256, "ESD_ISO")
	unknown.Volume.Created = built
	known := file("Windows11.iso", 4556128256, "ESD_ISO")
	known.Volume.Created = built

	res := &scan.Result{Files: []scan.File{unknown, known}}
	st := state.New(scan.Proxmox)
	st.RecordScan(res, time.Now())
	if err := st.Assign("Windows11.iso", "windows-11-x64", "11"); err != nil {
		t.Fatal(err)
	}

	got := top(t, Suggest(res, st, cat, "Windows.iso"))
	if got.Entry.ID != "windows-11-x64" || got.Version != "11" {
		t.Fatalf("guessed %s %q, want windows-11-x64 11", got.Entry.ID, got.Version)
	}
	if !got.Sure() {
		t.Errorf("score %d: a twin should be a sure guess", got.Score)
	}
	if want := "same size as Windows11.iso"; !strings.Contains(got.Reason, want) {
		t.Errorf("reason %q should mention %q", got.Reason, want)
	}

	// A file of another size is not a twin, however alike the labels are.
	other := file("Other.iso", 4000000000, "ESD_ISO")
	other.Volume.Created = built
	res.Files = append(res.Files, other)
	for _, g := range Suggest(res, st, cat, "Other.iso") {
		if g.Sure() {
			t.Errorf("Other.iso: %s scored %d on %s", g.Entry.ID, g.Score, g.Reason)
		}
	}
}

// A checksum the catalog publishes beats everything else.
func TestSuggestFromKnownHash(t *testing.T) {
	cat := testCatalog(t)
	var hash, entryID string
	for i := range cat.Entries {
		if len(cat.Entries[i].KnownHashes) > 0 {
			hash, entryID = cat.Entries[i].KnownHashes[0], cat.Entries[i].ID
			break
		}
	}
	if hash == "" {
		t.Skip("no entry in the catalog publishes a known-good checksum yet")
	}

	f := file("mystery.iso", 1<<30, "")
	res := &scan.Result{Files: []scan.File{f}}
	st := state.New(scan.Proxmox)
	st.RecordScan(res, time.Now())
	rec := st.Files["mystery.iso"]
	rec.SHA256 = hash
	st.Files["mystery.iso"] = rec

	got := top(t, Suggest(res, st, cat, "mystery.iso"))
	if got.Entry.ID != entryID {
		t.Errorf("guessed %s, want %s", got.Entry.ID, entryID)
	}
	if got.Score < 99 {
		t.Errorf("score %d: a published checksum is certain", got.Score)
	}
}

// A file isoshelf has seen here before is recognized by its checksum, whatever
// it is called now.
func TestSuggestFromArchive(t *testing.T) {
	cat := testCatalog(t)
	const hash = "8a7b2c0d1e2f30415263748596a7b8c9d0e1f2031425364758697a8b9cadbecf"

	// A netboot.xyz image that was removed from the folder.
	st := state.New(scan.Proxmox)
	first := &scan.Result{Files: []scan.File{file("netboot.xyz.iso", 700<<20, "")}}
	st.RecordScan(first, time.Now())
	rec := st.Files["netboot.xyz.iso"]
	rec.SHA256, rec.Entry, rec.Version = hash, "netbootxyz", "2.0.78"
	st.Files["netboot.xyz.iso"] = rec
	st.Archived("netboot.xyz.iso", state.GoneRemoved, time.Now())

	// It comes back under a name nothing recognizes.
	second := &scan.Result{Files: []scan.File{file("boot-thing.iso", 700<<20, "")}}
	st.RecordScan(second, time.Now())
	back := st.Files["boot-thing.iso"]
	back.SHA256 = hash
	st.Files["boot-thing.iso"] = back

	got := top(t, Suggest(second, st, cat, "boot-thing.iso"))
	if got.Entry.ID != "netbootxyz" || got.Version != "2.0.78" {
		t.Fatalf("guessed %s %q, want netbootxyz 2.0.78", got.Entry.ID, got.Version)
	}
	if !got.Sure() {
		t.Errorf("score %d: a file seen here before is a sure guess", got.Score)
	}
}

// Nothing in the catalog resembles a holiday video, so nothing is suggested.
func TestSuggestSaysNothingWhenItKnowsNothing(t *testing.T) {
	cat := testCatalog(t)
	res := &scan.Result{Files: []scan.File{file("holiday-2019.iso", 4<<30, "Holiday 2019")}}
	if guesses := Suggest(res, state.New(scan.Proxmox), cat, "holiday-2019.iso"); len(guesses) > 0 {
		t.Errorf("guessed %s (%s, score %d) for a holiday disc",
			guesses[0].Entry.ID, guesses[0].Reason, guesses[0].Score)
	}
}

// An ambiguous filename offers every entry it fits, instead of nothing.
func TestSuggestOffersAmbiguousMatches(t *testing.T) {
	cat := testCatalog(t)
	f := file("Win11_23H2_English_x64v2.iso", 6<<30, "CCCOMA_X64FRE_EN-US_DV9")
	f.Matches = cat.Match(f.Name())
	if len(f.Matches) < 2 {
		f.Matches = append(f.Matches, catalog.Match{Entry: &cat.Entries[0], Version: "1"},
			catalog.Match{Entry: &cat.Entries[1], Version: "2"})
	}
	res := &scan.Result{Files: []scan.File{f}}

	guesses := Suggest(res, state.New(scan.Proxmox), cat, f.Path)
	for _, m := range f.Matches {
		found := false
		for _, g := range guesses {
			found = found || g.Entry.ID == m.Entry.ID
		}
		if !found {
			t.Errorf("%s was not offered", m.Entry.ID)
		}
	}
}

func hasString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
