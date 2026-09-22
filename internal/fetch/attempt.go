package fetch

import (
	"context"
	"encoding"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

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
	// Headers for one particular host, set by whoever knows that host needs
	// them - at present another isoshelf on the network, which likes to know
	// which folder is asking. Never sent anywhere else: a header meant for
	// the machine next door has no business going to a project's mirror.
	if c.HostHeaders != nil {
		for name, value := range c.HostHeaders[httpReq.URL.Host] {
			httpReq.Header.Set(name, value)
		}
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

func restoreHash(digest hash.Hash, saved []byte) error {
	u, ok := digest.(encoding.BinaryUnmarshaler)
	if !ok || len(saved) == 0 {
		return errors.New("no saved hash state")
	}
	return u.UnmarshalBinary(saved)
}
