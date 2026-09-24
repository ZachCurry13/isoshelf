package remote

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Being a good neighbour to the projects' servers (#59). They are run by
// volunteers as often as not, and a tool that thousands of people point at
// them has to take "not now" for an answer.
//
//   - When a server says how long to wait - 429 Too Many Requests or 503
//     Service Unavailable with Retry-After - isoshelf waits that long, from
//     any server, not only GitHub.
//   - When the wait is longer than it will sit through, it stops and says so,
//     rather than retrying anyway. The next check, or Refresh, asks again.
//   - When a server doesn't say, the doubling wait between tries is spread a
//     little at random, so many copies of isoshelf that failed together don't
//     all come back in the same second.

// MaxPoliteWait is the longest isoshelf waits, when a server asks, before
// trying again. Somebody may be watching the page; past this it gives up for
// now rather than hold them up.
const MaxPoliteWait = time.Minute

// BusyError means a server asked isoshelf to come back later than it will
// wait.
type BusyError struct {
	URL  string
	Wait time.Duration
}

func (e *BusyError) Error() string {
	host := e.URL
	if u, err := url.Parse(e.URL); err == nil && u.Host != "" {
		host = u.Host
	}
	return fmt.Sprintf("%s is busy and asked isoshelf to come back in %s, so it will try again next time",
		host, roughly(e.Wait))
}

// retryAfter reads a Retry-After header: a number of seconds, or a date. Zero
// when there is none or it can't be read.
func retryAfter(h http.Header, now time.Time) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(v); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

// Refused turns an answer that isn't 200 into an error, reading how long the
// server asked isoshelf to wait: a *StatusError that says so, or a
// *BusyError when the wait is longer than isoshelf will sit through.
func Refused(target string, resp *http.Response, now time.Time) error {
	err := &StatusError{URL: target, Status: resp.Status, Code: resp.StatusCode}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		err.RetryAfter = retryAfter(resp.Header, now)
		if err.RetryAfter > MaxPoliteWait {
			return &BusyError{URL: target, Wait: err.RetryAfter}
		}
	}
	return err
}

// NextWait is how long to wait before trying again after err: what the
// server asked for, if it asked, or base spread at random between half and
// one and a half times itself.
func NextWait(err error, base time.Duration) time.Duration {
	var status *StatusError
	if errors.As(err, &status) && status.RetryAfter > 0 {
		return status.RetryAfter
	}
	if base <= 0 {
		return 0
	}
	return base/2 + rand.N(base)
}

// roughly says a duration the way a person would.
func roughly(d time.Duration) string {
	switch {
	case d < 2*time.Minute:
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	case d < 2*time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
}
