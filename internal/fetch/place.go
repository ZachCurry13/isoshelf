package fetch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// place verifies the finished download and renames it into the target.
func (c *Client) place(ctx context.Context, req Request, part, final, url string, st *downloadState, progress func(Progress)) (*Result, error) {
	result := &Result{Path: final, Size: st.size, SHA256: st.sha256, URL: url, Resumed: st.resumed}
	if req.Checksum != nil {
		progress(Progress{Stage: Verifying, Filename: req.Filename, Total: st.size, URL: url})
		var err error
		if req.Checksum.Algorithm == verify.SHA256 {
			if !strings.EqualFold(st.sha256, req.Checksum.Hex) {
				err = &verify.Mismatch{Name: req.Filename, Expected: *req.Checksum, Got: st.sha256}
			}
		} else {
			// Another algorithm: read the file back to compute it.
			err = verify.Check(ctx, part, *req.Checksum, func(done int64) {
				progress(Progress{Stage: Verifying, Filename: req.Filename, Done: done, Total: st.size, URL: url})
			})
		}
		if err != nil {
			var mismatch *verify.Mismatch
			if errors.As(err, &mismatch) {
				// A bad download is never kept: it would only be resumed into
				// the same wrong file later.
				os.Remove(part)
				os.Remove(part + ".json")
			}
			return nil, err
		}
		result.Verified = true
		result.Weak = req.Checksum.Algorithm.Weak()
	}

	// The project's signature over the image itself (#5), read from the
	// finished file. A file it doesn't match is thrown away like one whose
	// checksum doesn't: it can only ever be the wrong file.
	if req.Signature != nil {
		progress(Progress{Stage: Verifying, Filename: req.Filename, Total: st.size, URL: url})
		if err := checkImageSignature(part, req.Signature); err != nil {
			if errors.Is(err, verify.ErrBadSignature) {
				os.Remove(part)
				os.Remove(part + ".json")
			}
			return nil, err
		}
		result.Signed = true
	}

	progress(Progress{Stage: Placing, Filename: req.Filename, Done: st.size, Total: st.size, URL: url})
	if req.BeforePlace != nil {
		if err := req.BeforePlace(); err != nil {
			return nil, err
		}
	}
	if err := os.Rename(part, final); err != nil {
		return nil, err
	}
	os.Remove(part + ".json")
	return result, nil
}

// ImageSignature is a detached signature over the image and the key that
// must have made it.
type ImageSignature struct {
	Key *verify.Keyring
	Sig []byte
}

func checkImageSignature(part string, s *ImageSignature) error {
	f, err := os.Open(part)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := s.Key.CheckDetached(bufio.NewReaderSize(f, 1<<20), s.Sig); err != nil {
		return fmt.Errorf("the download isn't signed by the project's key, so it was thrown away: %w", err)
	}
	return nil
}
