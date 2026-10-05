package update

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A project that publishes only an MD5 (v0.8.10): the download is checked
// against it, so a damaged one would be thrown away, but a match proves
// nothing about who made the file. So it never replaces the old one, it
// isn't called verified, and its record says nothing checked it.
func TestAnMD5MatchIsWeak(t *testing.T) {
	dir, st := target(t)
	old := filepath.Join(dir, "example-1.iso")
	if err := os.WriteFile(old, []byte("the old image"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum([]byte(newImage))
	rc, fc := clients(t, site(t, hex.EncodeToString(sum[:])))
	res, err := Run(context.Background(), Options{
		Target: dir, Entry: testEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-1.iso"}, Removal: DeleteNow, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verified || !res.Weak {
		t.Errorf("verified=%v weak=%v, want a weak match that isn't called verified", res.Verified, res.Weak)
	}
	if _, err := os.Stat(old); err != nil {
		t.Error("the old file was removed on the word of an MD5")
	}
	if got := st.Files["example-2.iso"].Origin.Checked; got != "" {
		t.Errorf("the record says it was checked against %q", got)
	}
}
