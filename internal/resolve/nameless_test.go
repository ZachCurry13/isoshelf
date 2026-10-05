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
