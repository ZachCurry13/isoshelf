package upload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

func testNow() time.Time { return time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) }

func place(t *testing.T, dir, name, content string, opts Options) (*Result, error) {
	t.Helper()
	opts.Target, opts.Name, opts.Now = dir, name, testNow
	if opts.Profile == "" {
		opts.Profile = scan.Folder
	}
	if opts.State == nil {
		opts.State = state.New(scan.Folder)
	}
	return Place(context.Background(), strings.NewReader(content), opts)
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The ordinary case: a file arrives and is recorded, with nothing left behind.
func TestPlaceAddsTheFile(t *testing.T) {
	dir := t.TempDir()
	st := state.New(scan.Folder)
	res, err := place(t, dir, "ubuntu-24.04-desktop-amd64.iso", "an image", Options{State: st})
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "ubuntu-24.04-desktop-amd64.iso" || res.Size != int64(len("an image")) || res.Replaced != "" {
		t.Errorf("result = %+v", res)
	}
	if got := read(t, filepath.Join(dir, res.Name)); got != "an image" {
		t.Errorf("the file holds %q", got)
	}
	rec, ok := st.Files[res.Name]
	if !ok || rec.PlacedAt.IsZero() {
		t.Errorf("the file wasn't recorded: %+v", st.Files)
	}
	if rec.Entry != "" {
		t.Errorf("the upload guessed at what the image is (%q); the scan does that", rec.Entry)
	}
	// Nothing is left in the folder it arrived through.
	left, _ := os.ReadDir(filepath.Join(dir, state.DirName, incomingDir))
	if len(left) != 0 {
		t.Errorf("%d files left in incoming", len(left))
	}
}

// Only image files, decided the way the folder's own scan decides.
func TestPlaceRefusesWhatIsntAnImage(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"notes.txt", "setup.exe", "holiday.jpg", "archive.zip"} {
		if _, err := place(t, dir, name, "x", Options{}); !errors.Is(err, ErrNotAnImage) {
			t.Errorf("%s: %v, want ErrNotAnImage", name, err)
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s was written anyway", name)
		}
	}
	// A Proxmox folder lists fewer kinds than a plain folder does.
	if _, err := place(t, dir, "card.img.xz", "x", Options{Profile: scan.Proxmox}); !errors.Is(err, ErrNotAnImage) {
		t.Errorf("compressed image into Proxmox storage: %v, want ErrNotAnImage", err)
	}
	if _, err := place(t, dir, "card.img.xz", "x", Options{Profile: scan.Folder}); err != nil {
		t.Errorf("compressed image into a plain folder: %v", err)
	}
}

// A name with a path in it never reaches out of the folder.
func TestPlaceKeepsTheFileInTheFolder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"../escaped.iso", "../../escaped.iso", "/etc/escaped.iso",
		`..\escaped.iso`, "sub/nested.iso", "", "   ", ".isoshelf",
	} {
		res, err := place(t, dir, name, "x", Options{})
		if err != nil {
			continue // refused outright, which is fine
		}
		// Or accepted, but only ever as a plain name in the top of the folder.
		if strings.ContainsAny(res.Name, `/\`) {
			t.Errorf("%q was placed as %q", name, res.Name)
		}
		if _, err := os.Stat(filepath.Join(dir, res.Name)); err != nil {
			t.Errorf("%q landed somewhere other than the folder: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped.iso")); err == nil {
		t.Error("a file landed outside the folder")
	}
}

// Nothing is overwritten unless the user said what should happen to the file
// that's there.
func TestPlaceWontOverwriteWithoutAnAnswer(t *testing.T) {
	dir := t.TempDir()
	name := "netboot.xyz.iso"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("the one they have"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := place(t, dir, name, "the new one", Options{}); !errors.Is(err, ErrExists) {
		t.Errorf("%v, want ErrExists", err)
	}
	if got := read(t, filepath.Join(dir, name)); got != "the one they have" {
		t.Errorf("the old file was touched: %q", got)
	}
	if _, err := place(t, dir, name, "x", Options{Replace: update.Keep}); !errors.Is(err, ErrExists) {
		t.Errorf(`"keep" is not an answer to this question: %v`, err)
	}

	// Archive the old one: it waits in .isoshelf/removed and can come back.
	st := state.New(scan.Folder)
	res, err := place(t, dir, name, "the new one", Options{Replace: update.MoveAside, State: st})
	if err != nil {
		t.Fatal(err)
	}
	if res.Replaced != name {
		t.Errorf("result = %+v", res)
	}
	if got := read(t, filepath.Join(dir, name)); got != "the new one" {
		t.Errorf("the new file isn't in place: %q", got)
	}
	aside := filepath.Join(dir, state.DirName, update.RemovedDir, name)
	if got := read(t, aside); got != "the one they have" {
		t.Errorf("the old file isn't in the archive: %q", got)
	}
	if len(st.Past) != 1 || st.Past[0].Path != name {
		t.Errorf("the archive wasn't recorded: %+v", st.Past)
	}
}

// Replacing for real deletes the old one, because that is what was chosen.
func TestPlaceCanReplaceOutright(t *testing.T) {
	dir := t.TempDir()
	name := "netboot.xyz.iso"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := place(t, dir, name, "new", Options{Replace: update.DeleteNow}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, name)); got != "new" {
		t.Errorf("the file holds %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, state.DirName, update.RemovedDir, name)); err == nil {
		t.Error("the old file was archived, though deleting it is what was chosen")
	}
}

// An upload that stops halfway costs the user nothing: not the file they had,
// and not a stray part-file the next scan would puzzle over.
func TestPlaceThatFailsLeavesTheFolderAlone(t *testing.T) {
	dir := t.TempDir()
	name := "netboot.xyz.iso"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("the one they have"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Folder)
	_, err := Place(context.Background(), failingReader{}, Options{
		Target: dir, Name: name, Profile: scan.Folder,
		Replace: update.MoveAside, State: st, Now: testNow,
	})
	if err == nil {
		t.Fatal("a broken upload was reported as having worked")
	}
	if got := read(t, filepath.Join(dir, name)); got != "the one they have" {
		t.Errorf("the old file was lost or changed: %q", got)
	}
	if len(st.Past) != 0 {
		t.Errorf("the old file was recorded as gone: %+v", st.Past)
	}
	left, _ := os.ReadDir(filepath.Join(dir, state.DirName, incomingDir))
	if len(left) != 0 {
		t.Errorf("%d part-files left behind", len(left))
	}
}

// A file bigger than the room left is refused before it starts arriving.
func TestPlaceRefusesWhatWontFit(t *testing.T) {
	dir := t.TempDir()
	opts := Options{Size: 8 << 30, Room: space.Usage{Free: 1 << 30, Total: 64 << 30}}
	if _, err := place(t, dir, "big.iso", "x", opts); !errors.Is(err, ErrNoRoom) {
		t.Errorf("%v, want ErrNoRoom", err)
	}
	// A filesystem that doesn't say how much room it has doesn't block it.
	opts.Room = space.Usage{}
	if _, err := place(t, dir, "big.iso", "x", opts); err != nil {
		t.Errorf("unknown free space refused the upload: %v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("the connection dropped")
}
