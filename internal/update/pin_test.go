package update

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// A pinned file stays exactly where it is, whatever the answer for old files
// says: the update replaces the others and leaves this one alone.
func TestAPinnedFileIsNeverReplaced(t *testing.T) {
	dir, st := target(t)
	if err := os.WriteFile(filepath.Join(dir, "example-0.iso"), []byte("the build I keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, fc := clients(t, site(t, newImageSHA256()))

	res, err := Run(context.Background(), Options{
		Target: dir, Entry: testEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-0.iso", "example-1.iso"}, Pinned: []string{"example-0.iso"},
		Removal: DeleteNow, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "example-0.iso")); err != nil || string(got) != "the build I keep" {
		t.Errorf("the pinned file changed: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "example-1.iso")); !os.IsNotExist(err) {
		t.Error("the unpinned old file wasn't replaced")
	}
	if !slices.Equal(res.Kept, []string{"example-0.iso"}) || !slices.Equal(res.Removed, []string{"example-1.iso"}) {
		t.Errorf("kept %v, removed %v", res.Kept, res.Removed)
	}
}

// An image whose filename never changes would land its new file on the
// pinned one. It goes beside it instead, with its version in its name, even
// when the answer for old files is to replace them.
func TestAPinnedFixedNameFileGetsItsUpdateBesideIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixed.iso"), []byte("the old image"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	if err := st.Placed(dir, "fixed.iso", state.FileRecord{Entry: "fixed", Version: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	rc, fc := clients(t, fixedSite(t))

	res, err := Run(context.Background(), Options{
		Target: dir, Entry: fixedEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"fixed.iso"}, Pinned: []string{"fixed.iso"},
		Removal: DeleteNow, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "fixed.iso")); err != nil || string(got) != "the old image" {
		t.Fatalf("the pinned file changed: %q, %v", got, err)
	}
	if res.File == "fixed.iso" || !strings.HasPrefix(res.File, "fixed-") {
		t.Errorf("the new file is %q; it should sit beside the pinned one under its own name", res.File)
	}
	if len(res.Removed) != 0 {
		t.Errorf("removed %v", res.Removed)
	}
}
