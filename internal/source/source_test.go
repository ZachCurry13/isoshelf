package source

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

func recordedClient() *remote.Client {
	c := remote.New("test")
	c.HTTP = &http.Client{Transport: remotetest.Recorded()}
	c.Backoff = time.Millisecond
	return c
}

func entry(t *testing.T, id string) *catalog.Entry {
	t.Helper()
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	e := cat.Entry(id)
	if e == nil {
		t.Fatalf("no entry %q in the default catalog", id)
	}
	return e
}

// TestLatestRecorded runs the default catalog's real entries against the
// responses recorded from their sites on 2026-09-17.
func TestLatestRecorded(t *testing.T) {
	tests := []struct {
		id          string
		wantVersion string
		wantCycle   string
		wantTag     string
	}{
		{"linuxmint-cinnamon", "22.3", "22.3", ""},
		{"mx-linux-xfce-x64", "25.2", "25", ""},
		{"mx-linux-xfce-x32", "23.6", "23", ""},
		{"centos-7-i386-minimal", "7 (2009)", "7", ""},
		{"netbootxyz", "3.0.3", "", "3.0.3"},
		{"netbootxyz-multiarch", "3.0.3", "", "3.0.3"},
		{"bazzite-deck-gnome", "44.20260916", "", "44.20260916"},
		{"popos-2204-intel", "58", "", ""},
		{"cachyos-desktop", "260809", "", ""},
		{"manjaro-xfce", "26.1.0", "", ""},
		{"clonezilla-alternative", "20260705", "", ""},
	}
	client := recordedClient()
	for _, tt := range tests {
		rel, err := Latest(context.Background(), client, entry(t, tt.id))
		if err != nil {
			t.Errorf("%s: %v", tt.id, err)
			continue
		}
		if rel.Version != tt.wantVersion || rel.Cycle != tt.wantCycle || rel.Tag != tt.wantTag {
			t.Errorf("%s: got version %q cycle %q tag %q, want %q %q %q",
				tt.id, rel.Version, rel.Cycle, rel.Tag, tt.wantVersion, tt.wantCycle, tt.wantTag)
		}
	}
}

func TestLatestGitHubAsset(t *testing.T) {
	rel, err := Latest(context.Background(), recordedClient(), entry(t, "netbootxyz"))
	if err != nil {
		t.Fatal(err)
	}
	want := Asset{
		Name:   "netboot.xyz.iso",
		URL:    "https://github.com/netbootxyz/netboot.xyz/releases/download/3.0.3/netboot.xyz.iso",
		SHA256: "2206b05c1c7a8ec7a7a4356dd5f811e19c8081d7d5e2bc856f1f3f4059c5fed1",
	}
	if rel.Asset == nil {
		t.Fatal("no asset")
	}
	got := *rel.Asset
	got.Size = 0
	if got != want {
		t.Errorf("asset = %+v, want %+v", got, want)
	}
	if rel.Asset.Size <= 0 {
		t.Errorf("asset size = %d", rel.Asset.Size)
	}
}

func TestCycleFor(t *testing.T) {
	rel, err := Latest(context.Background(), recordedClient(), entry(t, "mx-linux-xfce-x64"))
	if err != nil {
		t.Fatal(err)
	}
	for v, want := range map[string]struct {
		cycle string
		eol   bool
	}{
		"21.3": {"21", true},
		"25.2": {"25", false},
		"25":   {"25", false},
		"2":    {"", false},
		"250":  {"", false},
	} {
		c := rel.CycleFor(v)
		switch {
		case want.cycle == "" && c != nil:
			t.Errorf("CycleFor(%q) = %q, want none", v, c.Name)
		case want.cycle != "" && (c == nil || c.Name != want.cycle || c.EOL != want.eol):
			t.Errorf("CycleFor(%q) = %+v, want cycle %q eol %v", v, c, want.cycle, want.eol)
		}
	}
}

// fakeEndOfLife serves an endoflife.date response in which LMDE 8 is newer
// than every numbered Linux Mint release.
func fakeEndOfLife() *remote.Client {
	c := remote.New("test")
	c.HTTP = &http.Client{Transport: remotetest.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result": {"releases": [
			{"name": "lmde8", "releaseDate": "2027-10-01", "isLts": false, "isEol": false, "eolFrom": null, "latest": null},
			{"name": "22.3", "releaseDate": "2026-01-11", "isLts": true, "isEol": false, "eolFrom": "2029-04-30", "latest": null},
			{"name": "21.3", "releaseDate": "2024-01-10", "isLts": true, "isEol": true, "eolFrom": "2027-04-30",
			 "latest": {"name": "21.3.1", "link": "https://example.org/21.3.1"}}
		]}}`))
	}))}
	return c
}

func TestEndOfLifeChannels(t *testing.T) {
	tests := []struct {
		channel, cycles string
		wantCycle       string
		wantVersion     string
		wantErr         bool
	}{
		{"latest", `\d+(?:\.\d+)*`, "22.3", "22.3", false},
		{"latest", "", "lmde8", "lmde8", false}, // what the cycles filter prevents
		{"lts", "", "22.3", "22.3", false},
		{"21.3", "", "21.3", "21.3.1", false},
		{"20.1", "", "", "", true},
	}
	for _, tt := range tests {
		e := &catalog.Entry{Source: catalog.Source{Type: catalog.SourceEndOfLife, Product: "linuxmint", Channel: tt.channel, Cycles: tt.cycles}}
		rel, err := Latest(context.Background(), fakeEndOfLife(), e)
		if tt.wantErr {
			if err == nil {
				t.Errorf("channel %q: want an error", tt.channel)
			}
			continue
		}
		if err != nil {
			t.Errorf("channel %q: %v", tt.channel, err)
			continue
		}
		if rel.Cycle != tt.wantCycle || rel.Version != tt.wantVersion {
			t.Errorf("channel %q cycles %q: got %q/%q, want %q/%q", tt.channel, tt.cycles, rel.Cycle, rel.Version, tt.wantCycle, tt.wantVersion)
		}
		if tt.cycles != "" && len(rel.Cycles) != 2 {
			t.Errorf("cycles filter kept %d cycles, want 2", len(rel.Cycles))
		}
	}
}

func TestLatestErrors(t *testing.T) {
	client := recordedClient()
	ctx := context.Background()

	if _, err := Latest(ctx, client, entry(t, "hirens-bootcd-pe")); !errors.Is(err, ErrManual) {
		t.Errorf("manual entry: got %v, want ErrManual", err)
	}

	unrecorded := &catalog.Entry{Source: catalog.Source{Type: catalog.SourceEndOfLife, Product: "nope", Channel: "latest"}}
	var status *remote.StatusError
	if _, err := Latest(ctx, client, unrecorded); !errors.As(err, &status) || status.Code != http.StatusNotFound {
		t.Errorf("unknown product: got %v, want a 404", err)
	}

	noTag := &catalog.Entry{Source: catalog.Source{Type: catalog.SourceGitHub, Repo: "netbootxyz/netboot.xyz", Tag: `v\d+`}}
	if _, err := Latest(ctx, client, noTag); err == nil || !strings.Contains(err.Error(), "no release") {
		t.Errorf("no matching tag: got %v", err)
	}

	noAsset := &catalog.Entry{Source: catalog.Source{Type: catalog.SourceGitHub, Repo: "netbootxyz/netboot.xyz", Tag: `.*`, Asset: `nothing\.iso`}}
	if _, err := Latest(ctx, client, noAsset); err == nil || !strings.Contains(err.Error(), "asset matching") {
		t.Errorf("no matching asset: got %v", err)
	}

	noVersion := &catalog.Entry{Source: catalog.Source{Type: catalog.SourceListing, URL: "https://mirror.cachyos.org/ISO/desktop/", Regex: `nope-(?P<version>\d+)`}}
	if _, err := Latest(ctx, client, noVersion); err == nil || !strings.Contains(err.Error(), "no version") {
		t.Errorf("listing without a match: got %v", err)
	}

	static := &catalog.Entry{Source: catalog.Source{Type: catalog.SourceStatic, Version: "1.2"}}
	if rel, err := Latest(ctx, client, static); err != nil || rel.Version != "1.2" {
		t.Errorf("static: got %v, %v", rel, err)
	}
}
