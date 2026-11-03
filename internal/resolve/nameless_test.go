package resolve

import (
	"context"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/source"
)

// A checksum file that names no file (v0.8.10) leaves the image's name to
// what the listing matched - AnduinOS's download page names the ISO, its
// CDN can't be listed, and its checksum file says only "SHA256: <hash>".
func TestAChecksumFileThatNamesNoFile(t *testing.T) {
	files := map[string]string{
		"example.org/isos/Example-2.0.4-amd64.sha256": "SHA256: " + hash12 + "\n",
	}
	e := loadEntry(t, `base = "https://example.org/isos/"
file = 'Example-{version}-amd64\.iso'
manifest = "Example-{version}-amd64.sha256"`)
	rel := &source.Release{Version: "2.0.4", Matched: "https://example.org/isos/Example-2.0.4-amd64.iso"}
	a, err := Resolve(context.Background(), client(site(files)), e, rel)
	if err != nil {
		t.Fatal(err)
	}
	if a.Filename != "Example-2.0.4-amd64.iso" || a.Checksum == nil || a.Checksum.Hex != hash12 {
		t.Errorf("got %s with %+v", a.Filename, a.Checksum)
	}
}

// AnduinOS's page links the checksum file, not the image (the image comes from
// a "thank you" page), so the listing matches the .sha256 and the name has to
// come from the pattern: once the version is filled in, it spells one file.
func TestANamelessChecksumForAFileTheVersionNames(t *testing.T) {
	files := map[string]string{
		"example.org/isos/Example-2.0.5-amd64.sha256": "SHA256: " + hash12 + "\n",
	}
	e := loadEntry(t, `base = "https://example.org/isos/"
file = 'Example-{version}-amd64\.iso'
manifest = "Example-{version}-amd64.sha256"`)
	rel := &source.Release{Version: "2.0.5", Matched: "https://example.org/Example-2.0.5-amd64.sha256"}
	a, err := Resolve(context.Background(), client(site(files)), e, rel)
	if err != nil {
		t.Fatal(err)
	}
	if a.Filename != "Example-2.0.5-amd64.iso" || a.Checksum == nil || a.Checksum.Hex != hash12 {
		t.Errorf("got %s with %+v", a.Filename, a.Checksum)
	}
}

// A pattern that is really a pattern is never guessed at: with no name in the
// checksum file and nothing else to go on, it fails, as before.
func TestANamelessChecksumForAPatternStillNeedsAName(t *testing.T) {
	files := map[string]string{
		"example.org/isos/Example-2.0.5-amd64.sha256": "SHA256: " + hash12 + "\n",
	}
	e := loadEntry(t, `base = "https://example.org/isos/"
file = 'Example-{version}-(?:amd64|arm64)\.iso'
manifest = "Example-{version}-amd64.sha256"`)
	rel := &source.Release{Version: "2.0.5", Matched: "https://example.org/Example-2.0.5-amd64.sha256"}
	if a, err := Resolve(context.Background(), client(site(files)), e, rel); err == nil {
		t.Errorf("guessed %s from a pattern that names two files", a.Filename)
	}
}
