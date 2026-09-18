package fetch

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/remote"
)

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
	if errors.As(err, &netErr) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	// Over HTTP/2 a busy or restarting server resets the stream or closes the
	// connection, and net/http doesn't export those error types. Seen with
	// repo.almalinux.org halfway through a download that resumed fine.
	msg := err.Error()
	for _, transient := range []string{"stopped after", "stream error:", "http2: server sent GOAWAY", "http2: client connection lost"} {
		if strings.Contains(msg, transient) {
			return true
		}
	}
	return false
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
