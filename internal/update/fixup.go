package update

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Make bootable (#3): a file the boot menu won't list, put into a shape it
// will. Two fixes, decided 2026-09-23: renaming (a ChromeOS-family .bin is a
// disk image Ventoy boots only as .img) and extracting (an image published
// compressed, as .xz or .gz). The catalog says which fix an image needs
// (fixup), and the file's own first bytes have to agree before anything is
// done: a name is a claim, the content is the fact. Every fix is asked for,
// one file at a time, and the original stays until the result is complete:
// a rename is one step that either happens or doesn't, and an extract is
// written beside the original and only takes its name once the whole of it
// has come out with the compressor's own check intact.

// Fix is one fix-up, worked out but not done yet.
type Fix struct {
	// Kind is "rename" or "extract".
	Kind string `json:"kind"`
	// From is the file as it is, To the name it will have; both relative to
	// the target, with forward slashes.
	From string `json:"from"`
	To   string `json:"to"`
	// Size is how big the result will be, or 0 when the file doesn't say.
	Size int64 `json:"size,omitempty"`
	// format is the compressor, for an extract.
	format sniff.Kind
}

// ErrNoFix means the catalog names no fix-up for the file, or the file's
// content doesn't match the fix-up it names.
var ErrNoFix = errors.New("no fix-up for this file")

// PlanFix works out how to make a file bootable, from what the catalog says
// it needs and what the file actually is.
func PlanFix(target, rel string, e *catalog.Entry) (*Fix, error) {
	full, err := insideTarget(target, rel)
	if err != nil {
		return nil, err
	}
	if e == nil || e.Fixup == "" {
		return nil, fmt.Errorf("%w: isoshelf doesn't know a fix for %s", ErrNoFix, path.Base(rel))
	}
	info, err := sniff.File(full)
	if err != nil {
		return nil, err
	}
	clean := path.Clean(filepath.ToSlash(rel))
	dir, name := path.Split(clean)
	fix := &Fix{From: clean}
	switch {
	case strings.HasPrefix(e.Fixup, "rename:"):
		ext := strings.TrimPrefix(e.Fixup, "rename:")
		if want := renameNeeds(ext); want != "" && info.Kind != want {
			return nil, fmt.Errorf("%w: %s doesn't start like a %s image (it looks like %s), so renaming it wouldn't make it boot", ErrNoFix, name, ext, info.Kind)
		}
		fix.Kind, fix.To = "rename", dir+strings.TrimSuffix(name, path.Ext(name))+ext
	case e.Fixup == "extract":
		switch info.Kind {
		case sniff.XZ, sniff.Gzip:
		case sniff.Zip, sniff.SevenZip:
			return nil, fmt.Errorf("%w: %s is a %s archive, which isoshelf can't unpack yet. Unpack it with your own tools.", ErrNoFix, name, info.Kind)
		default:
			return nil, fmt.Errorf("%w: %s isn't compressed (it looks like %s), so there is nothing to unpack", ErrNoFix, name, info.Kind)
		}
		inner := strings.TrimSuffix(name, path.Ext(name))
		if !isImageName(inner) {
			return nil, fmt.Errorf("%w: what's inside %s wouldn't be an image file (%s)", ErrNoFix, name, inner)
		}
		fix.Kind, fix.To, fix.format = "extract", dir+inner, info.Kind
		fix.Size = unpackedSize(full, info.Kind)
	default:
		return nil, fmt.Errorf("%w: isoshelf can't do %q yet", ErrNoFix, e.Fixup)
	}
	if fix.To == fix.From {
		return nil, fmt.Errorf("%w: %s already has that name", ErrNoFix, name)
	}
	if _, err := os.Lstat(filepath.Join(target, filepath.FromSlash(fix.To))); err == nil {
		return nil, fmt.Errorf("%s is already in the folder. Remove or rename it first, then try again.", fix.To)
	}
	return fix, nil
}

// renameNeeds is what a file must be for a new extension to be honest about
// it: an .img is a disk, an .iso an optical image.
func renameNeeds(ext string) sniff.Kind {
	switch ext {
	case ".img":
		return sniff.Disk
	case ".iso":
		return sniff.ISO
	}
	return ""
}

func isImageName(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	for _, e := range catalog.ImageExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// rename does a rename fix and moves the file's record with it: the bytes
// are the same, so everything known about them still holds.
func rename(target string, fix *Fix, st *state.State) error {
	to := filepath.Join(target, filepath.FromSlash(fix.To))
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("%s is already in the folder", fix.To)
	}
	if err := os.Rename(filepath.Join(target, filepath.FromSlash(fix.From)), to); err != nil {
		return err
	}
	rec := st.Files[fix.From]
	delete(st.Files, fix.From)
	return st.Placed(target, fix.To, rec)
}

// fixedOrigin is the origin of an unpacked file: the one it came out of,
// with how it arrived. A check of the compressed file carries over, because
// the compressor's own check proved the bytes came out as they went in.
func fixedOrigin(from string, was state.FileRecord, now time.Time) state.Origin {
	o := was.Origin
	o.How, o.From, o.At, o.Replaced = state.OriginExtract, from, now.UTC(), ""
	return o
}
