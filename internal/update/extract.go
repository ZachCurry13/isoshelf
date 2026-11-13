package update

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/ulikunitz/xz"

	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// ApplyFix does a fix worked out by PlanFix. After an extract, the
// compressed original is archived (MoveAside) or kept (Keep), as the user
// chose; it is never deleted here. progress, when set, hears how much has
// been written.
func ApplyFix(ctx context.Context, target string, fix *Fix, original Removal, st *state.State, now time.Time, progress func(done, total int64)) error {
	if fix.Kind == "rename" {
		return rename(target, fix, st)
	}
	if original != MoveAside && original != Keep {
		return fmt.Errorf("update: %q is not a way to leave the original", original)
	}
	if u, err := space.Of(target); err == nil && !u.Fits(fix.Size) {
		return fmt.Errorf("unpacking %s needs about %s and the folder has %s free. Make room, then try again.",
			path.Base(fix.From), sizeWords(fix.Size), sizeWords(u.Free))
	}
	if progress == nil {
		progress = func(int64, int64) {}
	}

	src, err := os.Open(filepath.Join(target, filepath.FromSlash(fix.From)))
	if err != nil {
		return err
	}
	defer src.Close()
	var r io.Reader
	switch fix.format {
	case sniff.Gzip:
		zr, err := gzip.NewReader(bufio.NewReaderSize(src, 1<<20))
		if err != nil {
			return err
		}
		zr.Multistream(true)
		r = zr
	case sniff.XZ:
		if r, err = xz.NewReader(bufio.NewReaderSize(src, 1<<20)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("update: can't unpack %s", fix.format)
	}

	// Written beside the folder's other unfinished files, and moved into
	// place only once every byte is out and the compressor's check passed.
	partDir := filepath.Join(target, state.DirName, fetch.PartialDir)
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		return err
	}
	part := filepath.Join(partDir, path.Base(fix.To)+".unpacking")
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, err := copyWatching(ctx, io.MultiWriter(out, hash), r, func(done int64) { progress(done, fix.Size) })
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(part)
		if errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("unpacking %s stopped after %s: %w. Nothing in the folder changed.", path.Base(fix.From), sizeWords(n), err)
	}

	to := filepath.Join(target, filepath.FromSlash(fix.To))
	if _, err := os.Lstat(to); err == nil {
		os.Remove(part)
		return fmt.Errorf("%s turned up in the folder while unpacking. Nothing was replaced.", fix.To)
	}
	if err := os.Rename(part, to); err != nil {
		os.Remove(part)
		return err
	}
	was := st.Files[fix.From]
	if err := st.Placed(target, fix.To, state.FileRecord{
		Entry: was.Entry, Version: was.Version, Assigned: was.Assigned,
		SHA256: hex.EncodeToString(hash.Sum(nil)), HashedAt: now.UTC(),
		PlacedAt: now.UTC(), Origin: fixedOrigin(fix.From, was, now),
	}); err != nil {
		return err
	}
	if original == MoveAside {
		return removeFile(target, fix.From, MoveAside, st, now)
	}
	return nil
}

// copyWatching copies like io.Copy, stopping when ctx ends and reporting
// how far it has got about every 8 MB.
func copyWatching(ctx context.Context, w io.Writer, r io.Reader, seen func(int64)) (int64, error) {
	buf := make([]byte, 1<<20)
	var n, told int64
	for {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		got, rerr := r.Read(buf)
		if got > 0 {
			if _, err := w.Write(buf[:got]); err != nil {
				return n, err
			}
			n += int64(got)
			if n-told >= 8<<20 {
				seen(n)
				told = n
			}
		}
		if rerr == io.EOF {
			seen(n)
			return n, nil
		}
		if rerr != nil {
			return n, rerr
		}
	}
}

func sizeWords(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%d MB", b>>20)
	}
	return fmt.Sprintf("%d KB", b>>10)
}
