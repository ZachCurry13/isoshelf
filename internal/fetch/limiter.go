package fetch

import (
	"context"
	"sync"
	"time"
)

// A Limiter caps how fast downloads may go, all of them together: the speed a
// person is willing to give isoshelf is for the whole of it, not for each
// download in it. Every download that shares one takes its turn at the same
// clock, so two running at once get about half each rather than the full
// amount each.
//
// It is a reservation clock rather than a bucket: taking n bytes books the
// next n/rate seconds, and waits until they start. There is no burst, which
// is what a cap on somebody's connection should mean, and the rate can be
// changed while downloads run (SetRate), which is what Settings does.
type Limiter struct {
	mu   sync.Mutex
	rate int64     // bytes a second; 0 means no limit
	next time.Time // when the next byte may be taken

	// now and sleep are replaced in tests.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// NewLimiter returns a limiter of bytesPerSecond, or none at all when it is 0.
func NewLimiter(bytesPerSecond int64) *Limiter {
	return &Limiter{rate: bytesPerSecond, now: time.Now, sleep: sleep}
}

// SetRate changes the limit for every download that shares l. 0 removes it.
func (l *Limiter) SetRate(bytesPerSecond int64) {
	l.mu.Lock()
	l.rate = bytesPerSecond
	l.mu.Unlock()
}

// Rate is the limit in bytes a second, or 0 when there is none.
func (l *Limiter) Rate() int64 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rate
}

// Wait books n bytes and waits for their turn. A nil limiter, or one with no
// limit, returns at once.
func (l *Limiter) Wait(ctx context.Context, n int) error {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.Lock()
	if l.rate <= 0 {
		l.mu.Unlock()
		return nil
	}
	now := l.now()
	if l.next.Before(now) {
		l.next = now // idle time is not saved up
	}
	wait := l.next.Sub(now)
	l.next = l.next.Add(time.Duration(float64(n) / float64(l.rate) * float64(time.Second)))
	l.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	return l.sleep(ctx, wait)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
