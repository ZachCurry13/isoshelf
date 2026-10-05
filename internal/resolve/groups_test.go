package resolve

import (
	"context"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/source"
)

// A checksum file named after something only the image's own name carries -
// Fedora's compose, "44-1.7" - is reached through a named group in
// artifact.file (v0.8.9, #6). The version comes from the release cycle, the
// compose from the file found in the folder.
func TestAChecksumNamedAfterAGroupInTheFile(t *testing.T) {
	files := map[string]string{
		"example.org/releases/44/iso/": `<a href="Example-Live-44-1.7.x86_64.iso">Example-Live-44-1.7.x86_64.iso</a>
<a href="Example-44-1.7-x86_64-CHECKSUM">Example-44-1.7-x86_64-CHECKSUM</a>`,
		"example.org/releases/44/iso/Example-44-1.7-x86_64-CHECKSUM": "SHA256 (Example-Live-44-1.7.x86_64.iso) = " + hash12 + "\n",
	}
	e := loadEntry(t, `base = "https://example.org/releases/{version}/iso/"
file = 'Example-Live-{version}-(?P<compose>[\d.]+)\.x86_64\.iso'
manifest = "Example-{version}-{compose}-x86_64-CHECKSUM"`)
	a, err := Resolve(context.Background(), client(site(files)), e, &source.Release{Version: "44"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Filename != "Example-Live-44-1.7.x86_64.iso" {
		t.Errorf("found %s", a.Filename)
	}
	if a.Checksum == nil || a.Checksum.Hex != hash12 || !strings.HasSuffix(a.ChecksumURL, "/Example-44-1.7-x86_64-CHECKSUM") {
		t.Errorf("checksum %+v from %s, want the compose's CHECKSUM file", a.Checksum, a.ChecksumURL)
	}
}
