// Package remote fetches small documents over HTTPS: API responses, download
// listings and checksum files. Large downloads belong to internal/fetch.
package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// MaxBody is the largest document Get reads.
const MaxBody = 8 << 20

// Client fetches documents. It is safe for concurrent use, and it remembers
// successful responses for its lifetime, so entries that share a source (two
// tracks of one GitHub repo, say) fetch it once. Use one Client per check.
type Client struct {
	// HTTP does the requests. Nil means a client with a 30-second timeout.
	HTTP *http.Client
	// UserAgent identifies isoshelf to the sites it asks.
	UserAgent string
	// GitHubToken, if set, is sent to api.github.com to raise the rate limit.
	GitHubToken string
	// Retries is how many more times to try after a timeout or server error.
	Retries int
	// Backoff is the wait before the first retry; it doubles each time.
	Backoff time.Duration

	mu    sync.Mutex
	cache map[string]*Response
}

// New returns a client for the given isoshelf version.
func New(appVersion string) *Client {
	return &Client{
		UserAgent: "isoshelf/" + appVersion + " (+https://github.com/ZachCurry13/isoshelf)",
		Retries:   2,
		Backoff:   time.Second,
	}
}

// Response is a fetched document.
type Response struct {
	Body []byte
	// URL is where the body came from after redirects.
	URL *url.URL
}

// StatusError is an HTTP response other than 200 OK.
type StatusError struct {
	URL    string
	Status string
	Code   int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET %s: %s", e.URL, e.Status)
}

// RateLimitError means GitHub refused the request until Reset.
type RateLimitError struct {
	Reset time.Time
}

func (e *RateLimitError) Error() string {
	if e.Reset.IsZero() {
		return "GitHub rate limit reached; try again later or set a GitHub token"
	}
	return fmt.Sprintf("GitHub rate limit reached; it resets at %s (or set a GitHub token)",
		e.Reset.Local().Format("15:04"))
}

var errInsecure = errors.New("only https:// is allowed")

// Get fetches rawURL. The URL and every redirect must use HTTPS. Timeouts and
// server errors are retried; other failures are not.
func (c *Client) Get(ctx context.Context, rawURL string) (*Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("GET %s: %w", rawURL, errInsecure)
	}

	c.mu.Lock()
	cached := c.cache[rawURL]
	c.mu.Unlock()
	if cached != nil {
		return cached, nil
	}

	wait := c.Backoff
	for attempt := 0; ; attempt++ {
		resp, err := c.get(ctx, u)
		if err == nil {
			c.mu.Lock()
			if c.cache == nil {
				c.cache = map[string]*Response{}
			}
			c.cache[rawURL] = resp
			c.mu.Unlock()
			return resp, nil
		}
		if attempt >= c.Retries || !retryable(err) || ctx.Err() != nil {
			return nil, err
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		wait *= 2
	}
}

func (c *Client) get(ctx context.Context, u *url.URL) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	if u.Host == "api.github.com" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if c.GitHubToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.GitHubToken)
		}
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if rl := rateLimit(u, resp); rl != nil {
			return nil, rl
		}
		return nil, &StatusError{URL: resp.Request.URL.String(), Status: resp.Status, Code: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxBody {
		return nil, fmt.Errorf("GET %s: response is larger than %d MiB", u, MaxBody>>20)
	}
	return &Response{Body: body, URL: resp.Request.URL}, nil
}

func (c *Client) httpClient() *http.Client {
	base := c.HTTP
	if base == nil {
		base = &http.Client{Timeout: 30 * time.Second}
	}
	client := *base
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("redirect to %s: %w", req.URL, errInsecure)
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	return &client
}

// rateLimit recognizes GitHub's rate-limit responses.
func rateLimit(u *url.URL, resp *http.Response) *RateLimitError {
	if u.Host != "api.github.com" || (resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests) {
		return nil
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		reset, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
		if reset > 0 {
			return &RateLimitError{Reset: time.Unix(reset, 0)}
		}
		return &RateLimitError{}
	}
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
		return &RateLimitError{Reset: time.Now().Add(time.Duration(secs) * time.Second)}
	}
	return nil
}

func retryable(err error) bool {
	var status *StatusError
	if errors.As(err, &status) {
		return status.Code >= 500 || status.Code == http.StatusTooManyRequests
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
