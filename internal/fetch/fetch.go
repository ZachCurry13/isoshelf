// Package fetch downloads images. Downloads resume where they left off,
// are staged inside the target folder so the finished file only has to be
// renamed into place, and are refused when they don't match the published
// checksum.
package fetch

import (
	"context"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// sidecar remembers an unfinished download next to its .part file.
type sidecar struct {
	URL      string    `json:"url"`
	ETag     string    `json:"etag,omitempty"`
	Offset   int64     `json:"offset"`
	Size     int64     `json:"size,omitempty"`
	HashSate []byte    `json:"sha256_state,omitempty"`
	Updated  time.Time `json:"updated"`
}

// Download fetches req and places the finished file in req.Dir. Mirrors are
// tried in turn when the official site fails. A checksum mismatch is never
// placed.
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
			return c.place(ctx, req, part, final, url, result, progress)
		}
		errs = append(errs, err)
		if ctx.Err() != nil || errors.As(err, new(*remote.RateLimitError)) {
			break
		}
	}
	return nil, errors.Join(errs...)
}

// download gets the file from one URL into part, resuming if it can.
func (c *Client) download(ctx context.Context, req Request, url, part string, progress func(Progress)) (*downloadState, error) {
	wait := c.Backoff
	var last error
	for attempt := 1; ; attempt++ {
		st, err := c.attempt(ctx, req, url, part, attempt, progress)
		if err == nil {
			return st, nil
		}
		last = err
		if attempt > c.Retries || !retryable(err) || ctx.Err() != nil {
			return nil, fmt.Errorf("%s: %w", url, last)
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		wait *= 2
	}
}

// downloadState is what one finished attempt produced.
type downloadState struct {
	size    int64
	sha256  string
	resumed bool
}

func (c *Client) attempt(ctx context.Context, req Request, url, part string, attempt int, progress func(Progress)) (*downloadState, error) {
	side := readSidecar(part)
	digest, offset := verify.MustNew(verify.SHA256), int64(0)
	resumed := false
	if side != nil && side.URL == url && side.Offset > 0 {
		if info, err := os.Stat(part); err == nil && info.Size() >= side.Offset {
			if err := restoreHash(digest, side.HashSate); err == nil {
				offset, resumed = side.Offset, true
			}
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", c.UserAgent)
	if strings.HasSuffix(httpReq.URL.Host, "github.com") && c.GitHubToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.GitHubToken)
	}
	if offset > 0 {
		httpReq.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
		if side.ETag != "" {
			httpReq.Header.Set("If-Range", side.ETag)
		}
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// The server ignored the range, or this is a fresh download.
		digest, offset, resumed = verify.MustNew(verify.SHA256), 0, false
	case http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		os.Remove(part)
		return nil, errors.New("the unfinished download no longer matches the file; it was discarded")
	default:
		if rl := rateLimit(resp); rl != nil {
			return nil, rl
		}
		return nil, &remote.StatusError{URL: url, Status: resp.Status, Code: resp.StatusCode}
	}

	total := req.Size
	if size := resp.ContentLength; size > 0 {
		total = offset + size
	}
	flags := os.O_CREATE | os.O_WRONLY
	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := file.Truncate(offset); err != nil {
		return nil, err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	side = &sidecar{URL: url, ETag: resp.Header.Get("ETag"), Size: total}
	report := throttle(progress, c.ProgressEvery)
	buf := make([]byte, 1<<20)
	lastSave := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			saveSidecar(part, side, offset, digest)
			return nil, err
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return nil, err
			}
			digest.Write(buf[:n])
			offset += int64(n)
			report(Progress{Stage: Downloading, Filename: req.Filename, Done: offset, Total: total, Resumed: resumed, URL: url, Attempt: attempt})
			if time.Since(lastSave) > 5*time.Second {
				saveSidecar(part, side, offset, digest)
				lastSave = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			saveSidecar(part, side, offset, digest)
			return nil, readErr
		}
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if total > 0 && offset != total {
		saveSidecar(part, side, offset, digest)
		return nil, fmt.Errorf("the download stopped after %d of %d bytes", offset, total)
	}
	progress(Progress{Stage: Downloading, Filename: req.Filename, Done: offset, Total: total, Resumed: resumed, URL: url, Attempt: attempt})
	return &downloadState{size: offset, sha256: hex.EncodeToString(digest.Sum(nil)), resumed: resumed}, nil
}

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

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	// No overall timeout: images are large. Stalled reads are caught by the
	// transport's own timeouts and by retries.
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}}
}

func readSidecar(part string) *sidecar {
	data, err := os.ReadFile(part + ".json")
	if err != nil {
		return nil
	}
	var s sidecar
	if json.Unmarshal(data, &s) != nil {
		return nil
	}
	return &s
}

func saveSidecar(part string, s *sidecar, offset int64, digest hash.Hash) {
	s.Offset, s.Updated = offset, time.Now().UTC()
	if m, ok := digest.(encoding.BinaryMarshaler); ok {
		if data, err := m.MarshalBinary(); err == nil {
			s.HashSate = data
		}
	}
	if data, err := json.Marshal(s); err == nil {
		os.WriteFile(part+".json", data, 0o644) // best effort: at worst we start over
	}
}

func restoreHash(digest hash.Hash, saved []byte) error {
	u, ok := digest.(encoding.BinaryUnmarshaler)
	if !ok || len(saved) == 0 {
		return errors.New("no saved hash state")
	}
	return u.UnmarshalBinary(saved)
}

// throttle limits how often progress is reported, but always passes stage
// changes through.
func throttle(progress func(Progress), every time.Duration) func(Progress) {
	var last time.Time
	var lastStage Stage
	return func(p Progress) {
		if p.Stage != lastStage || time.Since(last) >= every {
			last, lastStage = time.Now(), p.Stage
			progress(p)
		}
	}
}

func retryable(err error) bool {
	var status *remote.StatusError
	if errors.As(err, &status) {
		return status.Code >= 500 || status.Code == http.StatusTooManyRequests
	}
	if errors.As(err, new(*remote.RateLimitError)) {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) ||
		strings.Contains(err.Error(), "stopped after")
}

// rateLimit recognizes GitHub's rate-limit responses.
func rateLimit(resp *http.Response) *remote.RateLimitError {
	if resp.Header.Get("X-RateLimit-Remaining") != "0" {
		return nil
	}
	reset, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
	if reset > 0 {
		return &remote.RateLimitError{Reset: time.Unix(reset, 0)}
	}
	return &remote.RateLimitError{}
}
