package update

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// A download says where it came from and what proved it: the project's site,
// and the checksum the project publishes, with that checksum's address (#62).
func TestADownloadRecordsWhereItCameFromAndWhatProvedIt(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, newImageSHA256()))
	if _, err := Run(context.Background(), Options{
		Target: dir, Entry: testEntry(t), Client: rc, Fetcher: fc, State: st,
		Removal: Keep, Now: func() time.Time { return now },
	}); err != nil {
		t.Fatal(err)
	}
	got := st.Files["example-2.iso"].Origin
	if got.How != state.OriginDownload || !strings.HasSuffix(got.From, "/isos/example-2.iso") || !got.At.Equal(now) {
		t.Errorf("origin is %+v, want a download of example-2.iso at %v", got, now)
	}
	if !strings.HasSuffix(got.Checked, "/isos/SHA256SUMS") || !got.CheckedAt.Equal(now) {
		t.Errorf("checked against %q at %v, want the project's SHA256SUMS", got.Checked, got.CheckedAt)
	}
}
