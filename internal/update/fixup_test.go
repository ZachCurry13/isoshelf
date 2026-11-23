package update

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/ulikunitz/xz"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

func fixEntry(t *testing.T, fixup string) *catalog.Entry {
	t.Helper()
	text := `schema = 1
[[entry]]
id = "thing"
name = "Thing OS"
arch = "x86_64"
match = 'thing-(?P<version>\d+)\.(?:img|bin)(?:\.xz|\.gz)?'
samples = ["thing-1.img"]
fixup = "` + fixup + `"
page = "https://example.org/thing"
[entry.source]
type = "static"
version = "1"
`
	cat, err := catalog.Load(fstest.MapFS{"c.toml": {Data: []byte(text)}}, "c.toml")
	if err != nil {
		t.Fatal(err)
	}
	return cat.Entry("thing")
}

// disk is a small stand-in for a disk image: the MBR signature is what makes
// it one.
func disk() []byte {
	b := bytes.Repeat([]byte("disk image "), 20000)
	b[510], b[511] = 0x55, 0xaa
	return b
}

func put(t *testing.T, dir, name string, data []byte, st *state.State) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.Placed(dir, name, state.FileRecord{Entry: "thing", Version: "1"}); err != nil {
		t.Fatal(err)
	}
}

func TestExtractGzipAndXZ(t *testing.T) {
	for _, format := range []string{".gz", ".xz"} {
		dir, st := t.TempDir(), state.New(scan.Ventoy)
		var packed bytes.Buffer
		var w interface {
			Write([]byte) (int, error)
			Close() error
		}
		if format == ".gz" {
			w = gzip.NewWriter(&packed)
		} else {
			xw, err := xz.NewWriter(&packed)
			if err != nil {
				t.Fatal(err)
			}
			w = xw
		}
		w.Write(disk())
		w.Close()
		put(t, dir, "thing-1.img"+format, packed.Bytes(), st)

		fix, err := PlanFix(dir, "thing-1.img"+format, fixEntry(t, "extract"))
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if fix.To != "thing-1.img" || fix.Size != int64(len(disk())) {
			t.Errorf("%s: plan %+v, want thing-1.img of %d bytes", format, fix, len(disk()))
		}
		if err := ApplyFix(context.Background(), dir, fix, MoveAside, st, now, nil); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "thing-1.img")); !bytes.Equal(got, disk()) {
			t.Errorf("%s: the unpacked file isn't what went in", format)
		}
		if _, err := os.Stat(filepath.Join(dir, "thing-1.img"+format)); !os.IsNotExist(err) {
			t.Errorf("%s: the original is still in the folder, not archived", format)
		}
		rec := st.Files["thing-1.img"]
		if rec.Entry != "thing" || rec.SHA256 == "" || rec.Origin.How != state.OriginExtract || rec.Origin.From != "thing-1.img"+format {
			t.Errorf("%s: record %+v", format, rec)
		}
	}
}

func TestExtractKeepsTheOriginalWhenItBreaks(t *testing.T) {
	dir, st := t.TempDir(), state.New(scan.Ventoy)
	var packed bytes.Buffer
	w := gzip.NewWriter(&packed)
	w.Write(disk())
	w.Close()
	cut := packed.Bytes()[:packed.Len()-40]
	put(t, dir, "thing-1.img.gz", cut, st)
	fix, err := PlanFix(dir, "thing-1.img.gz", fixEntry(t, "extract"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyFix(context.Background(), dir, fix, MoveAside, st, now, nil); err == nil {
		t.Fatal("a damaged file unpacked without complaint")
	}
	if _, err := os.Stat(filepath.Join(dir, "thing-1.img")); !os.IsNotExist(err) {
		t.Error("half an image was left in the folder")
	}
	if _, err := os.Stat(filepath.Join(dir, "thing-1.img.gz")); err != nil {
		t.Error("the original went although the unpacked file never arrived")
	}
	parts, _ := os.ReadDir(filepath.Join(dir, state.DirName, "partial"))
	if len(parts) != 0 {
		t.Errorf("left behind: %v", parts)
	}
}

func TestRenameOnlyWhatTheContentBacksUp(t *testing.T) {
	dir, st := t.TempDir(), state.New(scan.Ventoy)
	put(t, dir, "thing-1.bin", disk(), st)
	fix, err := PlanFix(dir, "thing-1.bin", fixEntry(t, "rename:.img"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyFix(context.Background(), dir, fix, "", st, now, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "thing-1.img")); err != nil {
		t.Error("not renamed")
	}
	if st.Files["thing-1.img"].Entry != "thing" {
		t.Error("the record didn't move with the file")
	}

	put(t, dir, "thing-2.bin", []byte("just some text, not a disk"), st)
	if _, err := PlanFix(dir, "thing-2.bin", fixEntry(t, "rename:.img")); !errors.Is(err, ErrNoFix) {
		t.Errorf("a text file would be renamed to .img: %v", err)
	}
	put(t, dir, "thing-3.bin", disk(), st)
	put(t, dir, "thing-3.img", disk(), st)
	if _, err := PlanFix(dir, "thing-3.bin", fixEntry(t, "rename:.img")); err == nil {
		t.Error("a rename onto a file already there was planned")
	}
	if _, err := PlanFix(dir, "../thing-1.img", fixEntry(t, "rename:.img")); err == nil {
		t.Error("a file outside the folder was planned")
	}
}
