package update

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
	"github.com/ZachCurry13/isoshelf/internal/verify/verifytest"
)

// signedEntry is an entry whose project signs as shape says.
func signedEntry(t *testing.T, shape, sig string) *catalog.Entry {
	t.Helper()
	k := verifytest.NewKey("project")
	text := `schema = 1
[keys.project]
armored = '''
` + k.Armored + `'''
fingerprints = ["` + k.Fingerprint + `"]
[[entry]]
id = "example"
name = "Example Linux"
arch = "x86_64"
match = 'example-(?P<version>\d+)\.iso'
samples = ["example-1.iso"]
[entry.source]
type = "listing"
url = "https://example.org/api/latest"
regex = '"version": "(?P<version>\d+)"'
[entry.artifact]
base = "https://example.org/isos/"
file = 'example-{version}\.iso'
signature = "` + shape + `"
key = "project"
`
	if shape != catalog.SignatureImage {
		text += "manifest = \"SHA256SUMS\"\n"
	}
	if sig != "" {
		text += "sig = \"" + sig + "\"\n"
	}
	cat, err := catalog.Load(fstest.MapFS{"c.toml": {Data: []byte(text)}}, "c.toml")
	if err != nil {
		t.Fatal(err)
	}
	return cat.Entry("example")
}

func signedSite(files map[string][]byte) http.RoundTripper {
	return remotetest.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, filepath.Base(r.URL.Path), time.Unix(0, 0), strings.NewReader(string(body)))
	}))
}

// A signed checksum file is believed only with the project's signature on
// it; with a signature that doesn't match, nothing is downloaded at all.
func TestSignedChecksumFiles(t *testing.T) {
	project, stranger := verifytest.NewKey("project"), verifytest.NewKey("stranger")
	sums := []byte(newImageSHA256() + "  example-2.iso\n")
	for _, c := range []struct {
		name, shape, sig string
		files            map[string][]byte
		ok               bool
	}{
		{"detached", "manifest", "SHA256SUMS.gpg", map[string][]byte{"/isos/SHA256SUMS": sums, "/isos/SHA256SUMS.gpg": project.Detached(sums, false)}, true},
		{"someone else's", "manifest", "SHA256SUMS.gpg", map[string][]byte{"/isos/SHA256SUMS": sums, "/isos/SHA256SUMS.gpg": stranger.Detached(sums, false)}, false},
		{"clearsigned", "clearsigned", "", map[string][]byte{"/isos/SHA256SUMS": project.Clearsign(sums)}, true},
		{"not clearsigned", "clearsigned", "", map[string][]byte{"/isos/SHA256SUMS": sums}, false},
		{"image", "image", "{file}.sig", map[string][]byte{"/isos/example-2.iso.sig": project.Detached([]byte(newImage), true)}, true},
		{"image, wrong", "image", "{file}.sig", map[string][]byte{"/isos/example-2.iso.sig": project.Detached([]byte("other bytes"), true)}, false},
	} {
		c.files["/api/latest"] = []byte(`{"version": "2"}`)
		c.files["/isos/"] = []byte(`<a href="example-2.iso">example-2.iso</a>`)
		c.files["/isos/example-2.iso"] = []byte(newImage)
		dir, st := t.TempDir(), state.New(scan.Ventoy)
		rc, fc := clients(t, signedSite(c.files))
		res, err := Run(context.Background(), Options{
			Target: dir, Entry: signedEntry(t, c.shape, c.sig), Client: rc, Fetcher: fc, State: st, Now: func() time.Time { return now },
		})
		_, placed := os.Stat(filepath.Join(dir, "example-2.iso"))
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.ok && st.Files["example-2.iso"].Origin.Signed == "":
			t.Errorf("%s: the file's record doesn't say a signature proved it", c.name)
		case c.ok && !res.Verified:
			t.Errorf("%s: placed but not counted as verified", c.name)
		case !c.ok && !errors.Is(err, verify.ErrBadSignature):
			t.Errorf("%s: err = %v, want a bad signature", c.name, err)
		case !c.ok && placed == nil:
			t.Errorf("%s: a file with a bad signature was placed", c.name)
		}
	}
}
