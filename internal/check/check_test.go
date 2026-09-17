package check

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

const (
	hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func sha256Artifact(name, hex string) *resolve.Artifact {
	return &resolve.Artifact{Filename: name, Checksum: &verify.Checksum{Name: name, Algorithm: verify.SHA256, Hex: hex}}
}

func TestDecide(t *testing.T) {
	fixed := &catalog.Entry{ID: "fixed", FixedName: true, Source: catalog.Source{Type: catalog.SourceGitHub}}
	versioned := &catalog.Entry{ID: "versioned", Source: catalog.Source{Type: catalog.SourceListing}}
	eolLatest := &catalog.Entry{ID: "eol", Source: catalog.Source{Type: catalog.SourceEndOfLife, Channel: "latest"}}
	eolPinned := &catalog.Entry{ID: "pinned", Source: catalog.Source{Type: catalog.SourceEndOfLife, Channel: "7"}}
	cycles := []source.Cycle{{Name: "25"}, {Name: "21", EOL: true}, {Name: "7", EOL: true}}

	tests := []struct {
		name       string
		item       Item
		rel        *source.Release
		art        *resolve.Artifact
		err        error
		recorded   string
		wantStatus Status
		wantEOL    bool
		wantNote   string
	}{
		{"fixed name, same checksum", Item{Entry: fixed, Path: "a.iso"}, &source.Release{Version: "3.0.3"},
			sha256Artifact("a.iso", hashA), nil, hashA, UpToDate, false, ""},
		{"fixed name, checksum changed", Item{Entry: fixed, Path: "a.iso"}, &source.Release{Version: "3.0.3"},
			sha256Artifact("a.iso", hashA), nil, hashB, UpdateAvailable, false, "checksum changed"},
		{"fixed name, not hashed", Item{Entry: fixed, Path: "a.iso"}, &source.Release{Version: "3.0.3"},
			sha256Artifact("a.iso", hashA), nil, "", Unknown, false, "not hashed yet"},
		{"fixed name, md5 only", Item{Entry: fixed, Path: "a.iso"}, &source.Release{Version: "1"},
			&resolve.Artifact{Filename: "a.iso", Checksum: &verify.Checksum{Algorithm: verify.MD5, Hex: "x"}}, nil, hashA, Unknown, false, "md5"},
		{"fixed name, no checksum", Item{Entry: fixed, Path: "a.iso"}, &source.Release{Version: "1"},
			&resolve.Artifact{Filename: "a.iso"}, nil, hashA, Unknown, false, "no published checksum"},

		{"same file", Item{Entry: versioned, Path: "sub/x-58.iso", Version: "58"}, &source.Release{Version: "58"},
			sha256Artifact("x-58.iso", hashA), nil, "", UpToDate, false, ""},
		{"same file, hash differs", Item{Entry: versioned, Path: "x-58.iso", Version: "58"}, &source.Release{Version: "58"},
			sha256Artifact("x-58.iso", hashA), nil, hashB, ChecksumMismatch, false, "doesn't match"},
		{"newer file published", Item{Entry: versioned, Path: "x-56.iso", Version: "56"}, &source.Release{Version: "58"},
			sha256Artifact("x-58.iso", hashA), nil, "", UpdateAvailable, false, ""},
		{"check-only, newer version", Item{Entry: versioned, Path: "x-9.iso", Version: "9"}, &source.Release{Version: "10"},
			nil, nil, "", UpdateAvailable, false, ""},
		{"check-only, file is newest", Item{Entry: versioned, Path: "x-10.iso", Version: "10"}, &source.Release{Version: "10"},
			nil, nil, "", UpToDate, false, ""},

		{"old cycle is EOL", Item{Entry: eolLatest, Path: "mx-21.3.iso", Version: "21.3"}, &source.Release{Version: "25.2", Cycle: "25", Cycles: cycles},
			nil, nil, "", UpdateAvailable, true, ""},
		{"up to date but EOL", Item{Entry: eolPinned, Path: "centos-2009.iso", Version: "2009"}, &source.Release{Version: "7 (2009)", Cycle: "7", Cycles: cycles},
			sha256Artifact("centos-2009.iso", hashA), nil, "", EOL, true, ""},

		{"source failed", Item{Entry: versioned, Path: "x-1.iso", Version: "1", Status: NotChecked}, nil, nil,
			errors.New("GET https://example.org/: 503 Service Unavailable"), "", CheckFailed, false, "503"},
		{"missing entry keeps its status", Item{Entry: versioned, Status: Missing}, &source.Release{Version: "10"},
			nil, nil, "", Missing, false, ""},
		{"not bootable keeps its note", Item{Entry: versioned, Path: "x-1.bin", Version: "1", Status: NotBootable, Note: "ventoy lists only .iso"},
			nil, nil, errors.New("offline"), "", NotBootable, false, "ventoy lists only"},
	}
	for _, tt := range tests {
		it := tt.item
		decide(&it, tt.rel, tt.art, tt.err, tt.recorded)
		if it.Status != tt.wantStatus || it.EOL != tt.wantEOL || !strings.Contains(it.Note, tt.wantNote) {
			t.Errorf("%s: got status %q eol %v note %q, want %q %v containing %q",
				tt.name, it.Status, it.EOL, it.Note, tt.wantStatus, tt.wantEOL, tt.wantNote)
		}
	}
}

func TestSampleDrive(t *testing.T) {
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, f := range sampledrive.Files {
		if f.Name == "linuxmint-22.3-cinnamon-64bit.iso" {
			continue // deleted from the drive, to show up as missing
		}
		if err := os.WriteFile(filepath.Join(dir, f.Name), []byte("stand-in for "+f.Name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A second, older copy of CachyOS in a subfolder.
	if err := os.MkdirAll(filepath.Join(dir, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old", "cachyos-desktop-linux-251129.iso"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	res, err := scan.Scan(ctx, dir, cat, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	st.History = []state.ScanRecord{
		{Time: t0, Entries: []string{"linuxmint-cinnamon"}},
		{Time: t0, Entries: []string{"linuxmint-cinnamon"}},
	}
	st.RecordScan(res, t0.Add(time.Hour))
	if err := st.HashFiles(ctx, dir, st.NeedsHash(res, cat), nil); err != nil {
		t.Fatal(err)
	}

	report := Offline(res, st, cat)
	offline := map[string]Status{
		"FydeOS_for_PC_iris_v22.0-SP1-io.bin": NotBootable,
		"Windows.iso":                         Unrecognized,
		"HBCD_PE_x64.iso":                     Manual,
		"pop-os_22.04_amd64_intel_56.iso":     NotChecked,
		"":                                    Missing,
	}
	for path, want := range offline {
		if it := find(report, path); it == nil || it.Status != want {
			t.Errorf("offline %q: got %+v, want %s", path, it, want)
		}
	}

	client := remote.New("test")
	client.HTTP = &http.Client{Transport: remotetest.Recorded()}
	var lastDone, lastTotal int
	report.Online(ctx, client, st, func(done, total int) { lastDone, lastTotal = done, total })
	if lastDone == 0 || lastDone != lastTotal {
		t.Errorf("progress ended at %d of %d", lastDone, lastTotal)
	}
	if !report.Checked {
		t.Error("report not marked as checked")
	}

	online := []struct {
		path   string
		status Status
		latest string
		eol    bool
		note   string
	}{
		{"pop-os_22.04_amd64_intel_56.iso", UpdateAvailable, "58", false, ""},
		{"cachyos-desktop-linux-260308.iso", UpdateAvailable, "260809", false, ""},
		{"old/cachyos-desktop-linux-251129.iso", UpdateAvailable, "260809", false, "older copy"},
		{"MX-21.3_x64.iso", UpdateAvailable, "25.2", true, ""},
		{"MX-23.3_x32.iso", UpdateAvailable, "23.6", false, ""},
		{"manjaro-xfce-23.0.4-231015-linux65.iso", UpdateAvailable, "26.1.0", false, ""},
		{"clonezilla-live-20231102-mantic-amd64.iso", UpdateAvailable, "20260705", false, ""},
		{"CentOS-7-i386-Minimal-2009.iso", EOL, "2009", true, ""},
		// The stand-in files can't match the published checksums.
		{"netboot.xyz.iso", UpdateAvailable, "3.0.3", false, "checksum changed"},
		{"bazzite-deck-gnome-stable-live-amd64.iso", UpdateAvailable, "44.20260916", false, "checksum changed"},
		{"FydeOS_for_PC_iris_v22.0-SP1-io.bin", NotBootable, "", false, "Make bootable"},
		{"HBCD_PE_x64.iso", Manual, "", false, ""},
		{"Windows.iso", Unrecognized, "", false, ""},
		{"", Missing, "22.3", false, ""},
	}
	for _, tt := range online {
		it := find(report, tt.path)
		if it == nil {
			t.Errorf("%q: not in the report", tt.path)
			continue
		}
		if it.Status != tt.status || it.Latest != tt.latest || it.EOL != tt.eol || !strings.Contains(it.Note, tt.note) {
			t.Errorf("%q: got status %q latest %q eol %v note %q, want %q %q %v containing %q",
				tt.path, it.Status, it.Latest, it.EOL, it.Note, tt.status, tt.latest, tt.eol, tt.note)
		}
	}

	if got := report.Items[0].Status; got != UpdateAvailable {
		t.Errorf("first item is %q; updates should sort first", got)
	}
	// 11 manual entries, but Windows 11 has two files; the FydeOS .bin counts
	// as not bootable instead.
	if n := report.Counts()[Manual]; n != 12 {
		t.Errorf("%d manual items, want 12", n)
	}
}

// find returns the item with the given path; "" finds the first missing entry.
func find(r *Report, path string) *Item {
	for i := range r.Items {
		if r.Items[i].Path == path {
			return &r.Items[i]
		}
	}
	return nil
}
