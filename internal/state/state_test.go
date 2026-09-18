package state

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
)

var (
	t0 = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Hour)
)

func defaultCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// scanned builds a scan.File the way the scanner would, matching its name
// against the catalog.
func scanned(cat *catalog.Catalog, path string, size int64, mod time.Time) scan.File {
	return scan.File{Path: path, Size: size, ModTime: mod, Matches: cat.Match(filepath.Base(path))}
}

func TestLoadNewTarget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "template", "iso")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Profile != scan.Proxmox {
		t.Errorf("profile = %q, want the suggested %q", s.Profile, scan.Proxmox)
	}
	if !targetIDPattern.MatchString(s.TargetID) {
		t.Errorf("target id %q doesn't look valid", s.TargetID)
	}
	if _, err := os.Stat(filepath.Join(dir, DirName)); !errors.Is(err, os.ErrNotExist) {
		t.Error("Load wrote to the target")
	}
}

func TestSaveAndLoad(t *testing.T) {
	cat := defaultCatalog(t)
	dir := t.TempDir()

	s := New(scan.Ventoy)
	s.RecordScan(&scan.Result{Files: []scan.File{scanned(cat, "netboot.xyz.iso", 100, t0)}}, t1)
	s.SetTrack("netbootxyz", Track{KeepOld: true})
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	// Saving twice replaces the file and leaves no temporary files behind.
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, DirName))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Errorf(".isoshelf holds %v, want only state.json", entries)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetID != s.TargetID || got.Profile != scan.Ventoy {
		t.Errorf("loaded id %q profile %q, want %q %q", got.TargetID, got.Profile, s.TargetID, scan.Ventoy)
	}
	if rec := got.Files["netboot.xyz.iso"]; rec.Entry != "netbootxyz" || !rec.ModTime.Equal(t0) {
		t.Errorf("record = %+v", rec)
	}
	if !got.Track("netbootxyz").KeepOld {
		t.Error("keep_old setting was lost")
	}
	if len(got.History) != 1 || !got.History[0].Time.Equal(t1) {
		t.Errorf("history = %v", got.History)
	}
}

func TestLoadRejectsBadState(t *testing.T) {
	tests := map[string]string{
		"newer version": `{"version": 2, "target_id": "ABCDEFGHIJ", "profile": "ventoy"}`,
		"bad target id": `{"version": 1, "target_id": "../../evil", "profile": "ventoy"}`,
		"bad profile":   `{"version": 1, "target_id": "ABCDEFGHIJ", "profile": "usb"}`,
		"not json":      `{"version": 1,`,
	}
	for name, content := range tests {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, DirName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, DirName, "state.json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestRecordScan(t *testing.T) {
	cat := defaultCatalog(t)
	s := New(scan.Ventoy)
	s.RecordScan(&scan.Result{Files: []scan.File{
		scanned(cat, "netboot.xyz.iso", 100, t0),
		scanned(cat, "linuxmint-22.3-cinnamon-64bit.iso", 200, t0),
		scanned(cat, "renamed.iso", 300, t0),
		scanned(cat, "Windows.iso", 400, t0),
		scanned(cat, "nas/HBCD_PE_x64.iso", 500, t0),
	}}, t0)
	s.setHash(scanned(cat, "netboot.xyz.iso", 100, t0), "aaaa", t0)
	s.setHash(scanned(cat, "Windows.iso", 400, t0), "bbbb", t0)
	s.setHash(scanned(cat, "nas/HBCD_PE_x64.iso", 500, t0), "cccc", t0)
	if err := s.Assign("renamed.iso", "cachyos-desktop", "260308"); err != nil {
		t.Fatal(err)
	}
	if err := s.Assign("gone.iso", "atlasos", ""); err == nil {
		t.Error("Assign to an unscanned file: want an error")
	}

	// Second scan: netboot.xyz is unchanged, Windows.iso was replaced by a
	// different file, Mint is gone, and the nas folder couldn't be read.
	s.RecordScan(&scan.Result{
		Files: []scan.File{
			scanned(cat, "netboot.xyz.iso", 100, t0),
			scanned(cat, "renamed.iso", 300, t0),
			scanned(cat, "Windows.iso", 999, t1),
		},
		Problems: []scan.Problem{{Path: "nas", Err: os.ErrPermission}},
	}, t1)

	// Nothing from the first scan says when it arrived: every file is new to
	// isoshelf then. Windows.iso changed since, so the second scan knows it
	// turned up in between.
	want := map[string]FileRecord{
		"netboot.xyz.iso":     {Size: 100, ModTime: t0, Entry: "netbootxyz", SHA256: "aaaa", HashedAt: t0},
		"renamed.iso":         {Size: 300, ModTime: t0, Entry: "cachyos-desktop", Version: "260308", Assigned: true},
		"Windows.iso":         {Size: 999, ModTime: t1, FirstSeen: t1},
		"nas/HBCD_PE_x64.iso": {Size: 500, ModTime: t0, Entry: "hirens-bootcd-pe", SHA256: "cccc", HashedAt: t0},
	}
	if len(s.Files) != len(want) {
		t.Errorf("records for %v, want %d", slices.Sorted(maps.Keys(s.Files)), len(want))
	}
	for path, w := range want {
		if got := s.Files[path]; got != w {
			t.Errorf("%s:\n got %+v\nwant %+v", path, got, w)
		}
	}

	last := s.History[len(s.History)-1]
	if wantIDs := []string{"cachyos-desktop", "netbootxyz"}; !slices.Equal(last.Entries, wantIDs) || last.Unrecognized != 1 {
		t.Errorf("history = %+v, want entries %v and 1 unrecognized", last, wantIDs)
	}
}

func TestHistoryIsCapped(t *testing.T) {
	s := New(scan.Ventoy)
	for i := range maxHistory + 5 {
		s.RecordScan(&scan.Result{}, t0.Add(time.Duration(i)*time.Minute))
	}
	if len(s.History) != maxHistory {
		t.Fatalf("history has %d scans, want %d", len(s.History), maxHistory)
	}
	if want := t0.Add(5 * time.Minute); !s.History[0].Time.Equal(want) {
		t.Errorf("oldest scan at %v, want %v", s.History[0].Time, want)
	}
}

func TestUsualSetAndMissing(t *testing.T) {
	s := New(scan.Ventoy)
	record := func(ids ...string) {
		s.History = append(s.History, ScanRecord{Time: t0, Entries: ids})
	}
	record("mint", "netboot", "tryout")
	record("mint", "netboot")
	record("mint")
	s.SetTrack("bazzite", Track{Starred: true})

	if got, want := s.UsualSet(), []string{"bazzite", "mint", "netboot"}; !slices.Equal(got, want) {
		t.Errorf("usual set = %v, want %v (tryout was seen only once)", got, want)
	}
	if got, want := s.Missing(), []string{"bazzite", "netboot"}; !slices.Equal(got, want) {
		t.Errorf("missing = %v, want %v", got, want)
	}

	// After 10 scans without it, an entry drops out of the usual set.
	for range usualWindow {
		record("mint")
	}
	if got, want := s.UsualSet(), []string{"bazzite", "mint"}; !slices.Equal(got, want) {
		t.Errorf("usual set later = %v, want %v", got, want)
	}

	s.SetTrack("bazzite", Track{})
	if _, ok := s.Tracks["bazzite"]; ok {
		t.Error("default track settings should be removed from the map")
	}
}

func TestHashing(t *testing.T) {
	cat := defaultCatalog(t)
	dir := t.TempDir()
	write := func(name, content string) scan.File {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return scanned(cat, name, info.Size(), info.ModTime())
	}
	netboot := write("netboot.xyz.iso", "hello")               // fixed name
	mint := write("linuxmint-22.3-cinnamon-64bit.iso", "mint") // versioned: not hashed
	hbcd := write("HBCD_PE_x64.iso", "hirens")                 // fixed name
	res := &scan.Result{Files: []scan.File{netboot, mint, hbcd}}

	s := New(scan.Ventoy)
	s.RecordScan(res, t0)
	need := s.NeedsHash(res, cat)
	if got, want := len(need), 2; got != want {
		t.Fatalf("NeedsHash returned %d files, want %d", got, want)
	}

	// A file that changed since the scan is hashed but not recorded.
	hbcdStale := hbcd
	hbcdStale.ModTime = hbcd.ModTime.Add(-time.Hour)
	s.Files[hbcdStale.Path] = FileRecord{Size: hbcdStale.Size, ModTime: hbcdStale.ModTime, Entry: "hirens-bootcd-pe"}

	var progressed int64
	err := s.HashFiles(context.Background(), dir, []scan.File{netboot, hbcdStale}, func(_ scan.File, done int64) { progressed += done })
	if err != nil {
		t.Fatal(err)
	}
	// sha256("hello")
	if got, want := s.Files["netboot.xyz.iso"].SHA256, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"; got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
	if got := s.Files["HBCD_PE_x64.iso"].SHA256; got != "" {
		t.Errorf("changed file got a hash: %q", got)
	}
	if progressed == 0 {
		t.Error("progress was never reported")
	}
	for _, f := range s.NeedsHash(res, cat) {
		if f.Path == netboot.Path {
			t.Error("netboot.xyz should no longer need a hash")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.HashFiles(ctx, dir, []scan.File{hbcd}, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled hashing: got %v, want context.Canceled", err)
	}
	if err := s.HashFiles(context.Background(), dir, []scan.File{scanned(cat, "missing.iso", 1, t0)}, nil); err == nil {
		t.Error("hashing a missing file: want an error")
	}
}

func TestMirrors(t *testing.T) {
	config := t.TempDir()
	if got, err := LoadMirrors(config); err != nil || got != nil {
		t.Fatalf("no mirrors yet: got %v, %v", got, err)
	}

	older, newer := New(scan.Ventoy), New(scan.Proxmox)
	older.History = []ScanRecord{{Time: t0, Entries: []string{"mint"}}, {Time: t0, Entries: []string{"mint"}}}
	newer.SetTrack("bazzite", Track{Starred: true})
	if err := older.SaveMirror(config, "E:\\", t0); err != nil {
		t.Fatal(err)
	}
	if err := newer.SaveMirror(config, "/var/lib/vz/template/iso", t1); err != nil {
		t.Fatal(err)
	}

	mirrors, err := LoadMirrors(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(mirrors) != 2 || mirrors[0].TargetID != newer.TargetID || mirrors[1].TargetID != older.TargetID {
		t.Fatalf("mirrors = %+v, want newest first", mirrors)
	}
	if got := mirrors[0].UsualSet(); !slices.Equal(got, []string{"bazzite"}) {
		t.Errorf("newer usual set = %v", got)
	}
	if got := mirrors[1].UsualSet(); !slices.Equal(got, []string{"mint"}) {
		t.Errorf("older usual set = %v", got)
	}

	bad := New(scan.Ventoy)
	bad.TargetID = "../escape"
	if err := bad.SaveMirror(config, "E:\\", t0); err == nil {
		t.Error("SaveMirror with a bad target id: want an error")
	}
}
