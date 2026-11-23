package web

import (
	"sync"
	"time"
)

// lockout slows down guessing. After tooMany wrong tries from one address,
// that address waits - a little at first, then longer. It is deliberately not
// a ban: locking somebody out of their own images for an hour because a
// phone auto-filled the wrong thing would be worse than the guessing.
const (
	tooMany     = 5
	firstWait   = 5 * time.Second
	longestWait = 5 * time.Minute
)

type attempts struct {
	mu     sync.Mutex
	failed map[string]*attempt
}

type attempt struct {
	count int
	until time.Time
	seen  time.Time
}

func newAttempts() *attempts { return &attempts{failed: map[string]*attempt{}} }

// wait says how long this address must wait before trying again, or 0.
func (a *attempts) wait(addr string, now time.Time) time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.failed[addr]
	if f == nil || now.After(f.until) {
		return 0
	}
	return f.until.Sub(now)
}

func (a *attempts) failure(addr string, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.failed[addr]
	if f == nil {
		f = &attempt{}
		a.failed[addr] = f
	}
	f.count++
	f.seen = now
	if f.count >= tooMany {
		wait := firstWait << min(f.count-tooMany, 6)
		f.until = now.Add(min(wait, longestWait))
	}
	// An address not heard from for an hour is forgotten, whatever its count
	// (the longest wait is five minutes, so nothing locked is lost), so this
	// map can't grow without end on a network where something keeps knocking,
	// and one address failing never wipes another's count.
	for other, old := range a.failed {
		if other != addr && now.Sub(old.seen) > time.Hour {
			delete(a.failed, other)
		}
	}
}

func (a *attempts) success(addr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.failed, addr)
}
