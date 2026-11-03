package web

import (
	"net/url"
	"slices"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/inventory"
)

// Which downloads run together (v0.10.0, #4). The queue keeps its order, and
// this decides how far down it a download may jump and how many go at once.
//
// Two rules do the politeness, and they are the whole of it:
//
//  1. Never two from the same server. A project's mirror sees one connection
//     from one person, however long the queue is. A waiting download whose
//     server is already busy is passed over for the next one that isn't, and
//     goes as soon as its server is free.
//  2. How many at once follows from the speed limit in Settings. With none,
//     two. With one, another starts only while the ones running are using
//     less of it than headroom allows, up to atOnceMost: a limit of 200
//     megabits that one slow mirror fills a fifth of is a limit worth
//     starting a second download under, and one it already fills is not.
//
// The speed is measured, not guessed: each run keeps its own speed over the
// last second or so (run.note).

const (
	// atOnceWithoutLimit is how many run together when no limit is set.
	atOnceWithoutLimit = 2
	// atOnceMost is the most that ever run together, whatever the limit.
	atOnceMost = 3
	// headroom is the share of the limit the running downloads may be using
	// before another is started. Under it there is room; over it there isn't.
	headroom = 0.7
	// settleFor is how long a download runs before its speed is believed.
	// Until then it is still ramping up, so nothing else starts.
	settleFor = 4 * time.Second
)

// hostOfLocked names the server a download will use, for rule 1. It is worked
// out from the catalog rather than the download, because it has to be known
// before the download starts: the folder the entry downloads from, GitHub for
// an entry that reads its release from there, or - for an image copied from
// another isoshelf on the network - that, which is a different machine from
// any project's. s.mu must be held; it reads s.cat, not s.catalog(), which
// takes the lock again.
func (s *Server) hostOfLocked(j *job) string {
	if j.copy != nil {
		return "your server"
	}
	if e := s.cat.Entry(j.entry); e != nil {
		if e.Artifact != nil {
			if u, err := url.Parse(e.Artifact.Base); err == nil && u.Host != "" {
				return u.Host
			}
		}
		if e.Source.Type == "github" {
			return "github.com"
		}
	}
	// Nothing known about where it comes from: treated as its own server, so
	// it runs when there is room rather than never.
	return "entry:" + j.entry
}

// atOnceWithoutLimit is how many run together when no limit is set. Tests set
// s.atOnce to run one at a time, which is what most of the queue's behaviour
// is easiest to see with.
func (s *Server) atOnceWithoutLimit() int {
	if s.atOnce > 0 {
		return s.atOnce
	}
	return atOnceWithoutLimit
}

// roomLocked says whether another download may start now. s.mu must be held.
func (s *Server) roomLocked() bool {
	n := len(s.running)
	if n == 0 {
		return true
	}
	if n >= atOnceMost {
		return false
	}
	limit := s.limiter.Rate()
	if limit <= 0 {
		return n < s.atOnceWithoutLimit()
	}
	now := s.cfg.Now()
	var using float64
	for _, r := range s.running {
		if now.Sub(r.started) < settleFor {
			return false // not measured yet: wait to see how fast it goes
		}
		using += r.speed
	}
	return using < headroom*float64(limit)
}

// nextLocked is the place in the queue of the download to start next: the
// first whose server isn't busy. -1 when every waiting download's server is.
func (s *Server) nextLocked() int {
	busy := map[string]bool{}
	for _, r := range s.running {
		busy[s.hostOfLocked(r.job)] = true
	}
	for i, j := range s.queue {
		if !busy[s.hostOfLocked(j)] {
			return i
		}
	}
	return -1
}

// note records how a download is going: where it has got to, and, about once
// a second, how fast. A restarted attempt starts counting from the top again,
// so a fall in the total is not a negative speed.
func (r *run) note(p inventory.Progress, now time.Time) {
	r.progress = p
	if r.sampleAt.IsZero() {
		r.sampleAt, r.sampleDone = now, p.Done
		return
	}
	if dt := now.Sub(r.sampleAt); dt >= time.Second {
		r.speed = max(0, float64(p.Done-r.sampleDone)/dt.Seconds())
		r.sampleAt, r.sampleDone = now, p.Done
	}
}

// removeRunLocked takes a finished download out of the running list.
func (s *Server) removeRunLocked(r *run) {
	s.running = slices.DeleteFunc(s.running, func(x *run) bool { return x == r })
}

// runOfLocked finds the running download with this id, or nil.
func (s *Server) runOfLocked(id int) *run {
	for _, r := range s.running {
		if r.job.id == id {
			return r
		}
	}
	return nil
}
