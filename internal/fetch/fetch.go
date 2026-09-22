// Package fetch downloads images. Downloads resume where they left off,
// are staged inside the target folder so the finished file only has to be
// renamed into place, and are refused when they don't match the published
// checksum.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// PartialDir is where unfinished downloads live, inside the target's
// .isoshelf folder. It is on the same filesystem as the target, so finishing
// a download is a rename, not a copy.
const PartialDir = "partial"

// Request is one download.
type Request struct {
	// URLs are where to get the file: the official site first, then mirrors.
	URLs []string
	// Filename is the name the file gets in Dir.
	Filename string
	// Dir is the target folder.
	Dir string
	// Size, when known, is the expected number of bytes.
	Size int64
	// Checksum, when set, must match before the file is placed.
	Checksum *verify.Checksum
	// Replace allows overwriting a file of the same name in Dir.
	Replace bool
	// BeforePlace, when set, runs after the download is verified and just
	// before the file is renamed into place. It is where the caller moves the
	// old file aside, so nothing is touched until the new file is known to be
	// good. If it fails, the file is not placed.
	BeforePlace func() error
}

// Stage says what a download is doing.
type Stage string

const (
	Downloading Stage = "downloading"
	Verifying   Stage = "verifying"
	Placing     Stage = "placing"
)

// Progress reports how a download is going.
type Progress struct {
	Stage Stage
	// Filename is the file being downloaded.
	Filename string
	// Done and Total are bytes.
	Done, Total int64
	// Resumed reports whether this download continued an earlier one.
	Resumed bool
	// URL is where the bytes are coming from.
	URL string
	// Attempt counts from 1 and rises when a download is retried.
	Attempt int
}

// Result describes a placed file.
type Result struct {
	Path   string
	Size   int64
	SHA256 string
	// Verified is true when a published checksum matched. False means the
	// project publishes none, so the file is "unverified".
	Verified bool
	// URL is where the file came from.
	URL string
	// Resumed reports whether an earlier, unfinished download was continued.
	Resumed bool
}

// Client downloads files.
type Client struct {
	// HTTP is the client to use; nil means one without a total timeout, so
	// large files aren't cut off.
	HTTP        *http.Client
	UserAgent   string
	GitHubToken string
	// HostHeaders are extra headers to send to one host and no other, keyed
	// by host:port. Another isoshelf on the network is the only user so far:
	// it likes to know which folder is asking, and that is nobody else's
	// business.
	HostHeaders map[string]map[string]string
	// Retries is how many more times to try after a network error or a
	// server error. 4xx responses are never retried.
	Retries int
	// Backoff is the wait before the first retry; it doubles each time.
	Backoff time.Duration
	// ProgressEvery limits how often progress is reported.
	ProgressEvery time.Duration
}

// New returns a client for the given isoshelf version.
func New(appVersion string) *Client {
	return &Client{
		UserAgent:     "isoshelf/" + appVersion + " (+https://github.com/ZachCurry13/isoshelf)",
		Retries:       4,
		Backoff:       2 * time.Second,
		ProgressEvery: 250 * time.Millisecond,
	}
}

// Download fetches req and places the finished file in req.Dir. The places in
// req.URLs are tried in turn, moving on when one fails to give the file or
// gives bytes that don't match the published checksum. A checksum mismatch is
// never placed, wherever it came from.
func (c *Client) Download(ctx context.Context, req Request, progress func(Progress)) (*Result, error) {
	if len(req.URLs) == 0 {
		return nil, errors.New("fetch: no URLs")
	}
	if req.Filename == "" || strings.ContainsAny(req.Filename, `/\`) {
		return nil, fmt.Errorf("fetch: %q is not a plain filename", req.Filename)
	}
	final := filepath.Join(req.Dir, req.Filename)
	if !req.Replace {
		if _, err := os.Stat(final); err == nil {
			return nil, fmt.Errorf("%s already exists", req.Filename)
		}
	}
	partDir := filepath.Join(req.Dir, state.DirName, PartialDir)
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		return nil, err
	}
	part := filepath.Join(partDir, req.Filename+".part")

	var errs []error
	for _, url := range req.URLs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(req.URLs) > 1 && req.Checksum == nil && url != req.URLs[0] {
			break // mirrors are only safe when the bytes can be checked
		}
		result, err := c.download(ctx, req, url, part, progress)
		if err == nil {
			placed, placeErr := c.place(ctx, req, part, final, url, result, progress)
			if placeErr == nil {
				return placed, nil
			}
			// Bytes that don't match the published checksum mean this copy is
			// wrong, not that the file can't be had: the next place is worth
			// trying, and whatever comes from it is checked just as hard.
			// The part file has already been thrown away, so nothing resumes
			// into the bad one. Any other failure here - a rename that won't
			// work, an old file that can't be moved - is about this machine
			// rather than the source, and trying elsewhere would only fail
			// the same way.
			if !errors.As(placeErr, new(*verify.Mismatch)) {
				return nil, placeErr
			}
			err = placeErr
		}
		errs = append(errs, err)
		if ctx.Err() != nil || errors.As(err, new(*remote.RateLimitError)) {
			break
		}
	}
	return nil, errors.Join(errs...)
}
