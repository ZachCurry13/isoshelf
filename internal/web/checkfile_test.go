package web

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/lastcheck"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// Check it reads a file nothing else would have hashed, and when its hash is
// the one the project publishes, the file is proven from then on: the report
// says which checksum it matched, and so do its records. Nothing goes online
// here - the project's answer is one isoshelf remembers.
func TestCheckItProvesAFileAgainstThePublishedChecksum(t *testing.T) {
	dirs, target := testDirs(t), t.TempDir()
	const name = "linuxmint-22.3-cinnamon-64bit.iso"
	body := []byte("an image")
	if err := os.WriteFile(filepath.Join(target, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	const published = "https://mirrors.edge.kernel.org/linuxmint/stable/22.3/sha256sum.txt"
	answers := lastcheck.Load(dirs.Config)
	answers.Remember("linuxmint-cinnamon", &source.Release{Version: "22.3"}, &resolve.Artifact{
		Filename:    name,
		Checksum:    &verify.Checksum{Name: name, Algorithm: verify.SHA256, Hex: hex.EncodeToString(sum[:])},
		ChecksumURL: published,
	})
	if err := answers.Save(); err != nil {
		t.Fatal(err)
	}

	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	before := itemAt(t, waitIdle(t, s), name)
	if before.Hashed || before.Origin.Checked != "" {
		t.Fatalf("before Check it: hashed=%v checked=%q, want neither", before.Hashed, before.Origin.Checked)
	}

	if rec := request(t, s, http.MethodPost, "/api/checkfile", map[string]string{"path": name}); rec.Code != http.StatusAccepted {
		t.Fatalf("Check it: %d %s", rec.Code, rec.Body)
	}
	after := itemAt(t, waitIdle(t, s), name)
	if after.Matched != published || after.Origin.Checked != published || after.Origin.CheckedAt.IsZero() {
		t.Errorf("after Check it: matched=%q checked=%q at %v, want both to name %s",
			after.Matched, after.Origin.Checked, after.Origin.CheckedAt, published)
	}

	// And the records keep it: a later scan, offline, still says so.
	request(t, s, http.MethodPost, "/api/scan", nil)
	if later := itemAt(t, waitIdle(t, s), name); later.Origin.Checked != published {
		t.Errorf("a later scan says checked=%q, want the proof kept", later.Origin.Checked)
	}
}

func itemAt(t *testing.T, st stateJSON, path string) check.ItemJSON {
	t.Helper()
	if st.Report != nil {
		for _, it := range st.Report.Items {
			if it.Path == path {
				return it
			}
		}
	}
	t.Fatalf("%s isn't in the report", path)
	return check.ItemJSON{}
}
