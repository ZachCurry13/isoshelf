package resolve

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

func client(rt http.RoundTripper) *remote.Client {
	c := remote.New("test")
	c.HTTP = &http.Client{Transport: rt}
	c.Backoff = time.Millisecond
	return c
}

// TestResolveRecorded resolves the default catalog's real entries against the
// responses recorded from their sites on 2026-09-17.
func TestResolveRecorded(t *testing.T) {
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		id       string
		filename string
		url      string
		sha256   string
	}{
		{"linuxmint-cinnamon", "linuxmint-22.3-cinnamon-64bit.iso",
			"https://mirrors.edge.kernel.org/linuxmint/stable/22.3/linuxmint-22.3-cinnamon-64bit.iso",
			"a081ab202cfda17f6924128dbd2de8b63518ac0531bcfe3f1a1b88097c459bd4"},
		{"popos-2204-intel", "pop-os_22.04_amd64_intel_58.iso",
			"https://iso.pop-os.org/22.04/amd64/intel/58/pop-os_22.04_amd64_intel_58.iso",
			"4e1c5e391062c79dc611ce383c2a709fecac36798ebd81444a734fd41252608e"},
		{"cachyos-desktop", "cachyos-desktop-linux-260809.iso",
			"https://mirror.cachyos.org/ISO/desktop/260809/cachyos-desktop-linux-260809.iso", ""},
		{"bazzite-deck-gnome", "bazzite-deck-gnome-stable-live-amd64.iso",
			"https://download.bazzite.gg/bazzite-deck-gnome-stable-live-amd64.iso", ""},
		{"centos-7-i386-minimal", "CentOS-7-i386-Minimal-2009.iso",
			"https://vault.centos.org/altarch/7.9.2009/isos/i386/CentOS-7-i386-Minimal-2009.iso",
			"bcbde5d345c5013fa618c38380765547be01a354883b3055f32d7067dd7b5bca"},
		{"netbootxyz", "netboot.xyz.iso",
			"https://github.com/netbootxyz/netboot.xyz/releases/download/3.0.3/netboot.xyz.iso",
			"2206b05c1c7a8ec7a7a4356dd5f811e19c8081d7d5e2bc856f1f3f4059c5fed1"},
	}
	c := client(remotetest.Recorded())
	for _, tt := range tests {
		e := cat.Entry(tt.id)
		rel, err := source.Latest(context.Background(), c, e)
		if err != nil {
			t.Errorf("%s: %v", tt.id, err)
			continue
		}
		a, err := Resolve(context.Background(), c, e, rel)
		if err != nil {
			t.Errorf("%s: %v", tt.id, err)
			continue
		}
		if a.Filename != tt.filename || len(a.URLs) != 1 || a.URLs[0] != tt.url {
			t.Errorf("%s: got %s at %v, want %s at %s", tt.id, a.Filename, a.URLs, tt.filename, tt.url)
		}
		if a.Checksum == nil || a.Checksum.Algorithm != verify.SHA256 {
			t.Errorf("%s: checksum = %+v, want SHA-256", tt.id, a.Checksum)
		} else if tt.sha256 != "" && a.Checksum.Hex != tt.sha256 {
			t.Errorf("%s: sha256 = %s, want %s", tt.id, a.Checksum.Hex, tt.sha256)
		}
	}

	for _, id := range []string{"mx-linux-xfce-x64", "manjaro-xfce", "hirens-bootcd-pe"} {
		rel := &source.Release{Version: "1"}
		if _, err := Resolve(context.Background(), c, cat.Entry(id), rel); !errors.Is(err, ErrNoArtifact) {
			t.Errorf("%s: got %v, want ErrNoArtifact", id, err)
		}
	}
}

// loadEntry builds a one-entry catalog around an [entry.artifact] table.
func loadEntry(t *testing.T, artifact string) *catalog.Entry {
	t.Helper()
	text := `schema = 1
[[entry]]
id = "example"
name = "Example"
arch = "x86_64"
match = 'example-(?P<version>\d+)\.iso'
samples = ["example-1.iso"]
[entry.source]
type = "listing"
url = "https://example.org/"
regex = '(?P<version>\d+)'
[entry.artifact]
` + artifact
	cat, err := catalog.Load(fstest.MapFS{"c.toml": {Data: []byte(text)}}, "c.toml")
	if err != nil {
		t.Fatal(err)
	}
	return cat.Entry("example")
}

// site serves a small fake download site.
func site(files map[string]string) http.RoundTripper {
	return remotetest.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if target, ok := strings.CutPrefix(files[r.Host+r.URL.Path], "redirect:"); ok {
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		body, ok := files[r.Host+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
}

const index = `<html><body>
<a href="../">../</a>
<a href="subdir/">subdir/</a>
<a href="example-7.iso">example-7.iso</a>
<a href="example-12.iso?download=1">example-12.iso</a>
<a href="example-12.iso.sha256">example-12.iso.sha256</a>
<a href="example-9.iso.zip">example-9.iso.zip</a>
</body></html>`

const hash12 = "1111111111111111111111111111111111111111111111111111111111111111"

func TestResolveFromIndex(t *testing.T) {
	rel := &source.Release{Version: "12"}
	files := map[string]string{
		"example.org/isos/":                      index,
		"example.org/isos/example-12.iso.sha256": hash12 + "  example-12.iso\n",
	}

	// The manifest name uses {file}, so the index is read first.
	e := loadEntry(t, `base = "https://example.org/isos/"
file = 'example-\d+\.iso'
manifest = "{file}.sha256"
mirrors = ["https://mirror.example.net/example/"]`)
	a, err := Resolve(context.Background(), client(site(files)), e, rel)
	if err != nil {
		t.Fatal(err)
	}
	wantURLs := []string{"https://example.org/isos/example-12.iso", "https://mirror.example.net/example/example-12.iso"}
	if a.Filename != "example-12.iso" || !slices.Equal(a.URLs, wantURLs) {
		t.Errorf("got %s at %v, want example-12.iso at %v", a.Filename, a.URLs, wantURLs)
	}
	if a.Checksum == nil || a.Checksum.Hex != hash12 || a.ChecksumURL != "https://example.org/isos/example-12.iso.sha256" {
		t.Errorf("checksum %+v from %s", a.Checksum, a.ChecksumURL)
	}

	// Without a manifest, the file is found in the index and left unverified.
	e = loadEntry(t, `base = "https://example.org/isos/"
file = 'example-\d+\.iso'`)
	a, err = Resolve(context.Background(), client(site(files)), e, rel)
	if err != nil {
		t.Fatal(err)
	}
	if a.Filename != "example-12.iso" || a.Checksum != nil {
		t.Errorf("got %s with checksum %+v, want example-12.iso unverified", a.Filename, a.Checksum)
	}
}

func TestResolveErrors(t *testing.T) {
	rel := &source.Release{Version: "12"}
	files := map[string]string{
		"example.org/isos/":            index,
		"example.org/isos/SHA256SUMS":  hash12 + "  example-12.iso\n",
		"example.org/moved/SHA256SUMS": "redirect:https://cdn.example.net/SHA256SUMS",
		"cdn.example.net/SHA256SUMS":   hash12 + "  example-12.iso\n",
	}
	tests := map[string]struct {
		artifact string
		want     string
	}{
		"checksum redirected to another host": {
			`base = "https://example.org/moved/"
file = 'example-{version}\.iso'
manifest = "SHA256SUMS"`,
			"checksums must come from the official site",
		},
		"file not in manifest": {
			`base = "https://example.org/isos/"
file = 'other-{version}\.iso'
manifest = "SHA256SUMS"`,
			"no file matching",
		},
		"file not in index": {
			`base = "https://example.org/isos/"
file = 'other-\d+\.iso'`,
			"no file matching",
		},
		"manifest missing": {
			`base = "https://example.org/isos/"
file = 'example-{version}\.iso'
manifest = "CHECKSUMS"`,
			"404",
		},
	}
	for name, tt := range tests {
		_, err := Resolve(context.Background(), client(site(files)), loadEntry(t, tt.artifact), rel)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: got %v, want an error containing %q", name, err, tt.want)
		}
	}
}

// TestEveryEntryRecorded checks that every entry of the built-in catalog that
// has a source resolves against the recorded responses. When it fails after a
// catalog change, refresh the recordings with
// "go run ./internal/remote/remotetest/record".
func TestEveryEntryRecorded(t *testing.T) {
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	c := client(remotetest.Recorded())
	for i := range cat.Entries {
		e := &cat.Entries[i]
		if e.Source.Type == catalog.SourceManual {
			continue
		}
		rel, err := source.Latest(context.Background(), c, e)
		if err != nil {
			t.Errorf("%s: %v", e.ID, err)
			continue
		}
		a, err := Resolve(context.Background(), c, e, rel)
		switch {
		case errors.Is(err, ErrNoArtifact):
			if e.Artifact != nil || e.Source.Asset != "" {
				t.Errorf("%s: has download information but resolved to nothing", e.ID)
			}
		case err != nil:
			t.Errorf("%s: %v", e.ID, err)
		case a.Checksum == nil:
			t.Errorf("%s: %s has no published checksum", e.ID, a.Filename)
		default:
			if _, ok := e.MatchName(a.Filename); !ok && e.Fixup == "" {
				t.Errorf("%s: the resolved file %s doesn't match the entry's own pattern", e.ID, a.Filename)
			}
		}
	}
}
