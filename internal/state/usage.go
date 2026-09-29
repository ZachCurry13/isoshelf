package state

import "time"

// What happened in a folder, week by week, for the usage page (v0.8.6).
//
// Everything here is worked out from records isoshelf already keeps: when
// each file arrived and how (Origin), when each image left and why (Past),
// and when each scan ran (History). Nothing new is recorded to make it, so
// nothing new can go stale, and a folder that has been in use for a month
// has a month of usage the first time somebody looks.
//
// It can only count what the records still hold. A file that arrived before
// v0.8.0 has no Origin and is counted by PlacedAt if isoshelf downloaded it,
// or not at all if it was found in the folder; an image that left before
// v0.8.6 has no Arrived, so its arrival is lost. Scan history keeps the last
// maxHistory scans. The page says so rather than presenting a partial count
// as a whole one.

// Tally is a number of files and the bytes they hold.
type Tally struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

func (t *Tally) add(size int64) {
	t.Files++
	t.Bytes += size
}

// Week is what happened in one folder in one week.
type Week struct {
	// Start is the Monday the week begins, at midnight where isoshelf runs.
	Start time.Time `json:"start"`
	// Downloaded came from the project's own site, Copied from another
	// isoshelf, and Added from somebody's own computer through the page.
	Downloaded Tally `json:"downloaded"`
	Copied     Tally `json:"copied"`
	Added      Tally `json:"added"`
	// Archived was set aside in .isoshelf/removed, Deleted removed for good,
	// Replaced taken out by an update, and Vanished went by other means.
	Archived Tally `json:"archived"`
	Deleted  Tally `json:"deleted"`
	Replaced Tally `json:"replaced"`
	Vanished Tally `json:"vanished"`
	// Scans is how many times the folder was read.
	Scans int `json:"scans"`
}

// Weeks returns the last n weeks up to and including the one holding now,
// newest first, in now's time zone.
func (s *State) Weeks(now time.Time, n int) []Week {
	if n <= 0 {
		return []Week{}
	}
	weeks := make([]Week, n)
	start := weekStart(now)
	for i := range weeks {
		weeks[i].Start = start.AddDate(0, 0, -7*i)
	}
	// which is the week t falls in, or nil when it is outside them all -
	// before the oldest, or after now because a clock was wrong.
	which := func(t time.Time) *Week {
		if t.IsZero() || t.After(now) {
			return nil
		}
		for i := range weeks {
			if !t.Before(weeks[i].Start) {
				return &weeks[i]
			}
		}
		return nil
	}

	arrived := func(o Origin, size int64) {
		w := which(o.At)
		if w == nil {
			return
		}
		switch o.How {
		case OriginDownload:
			w.Downloaded.add(size)
		case OriginCopy:
			w.Copied.add(size)
		case OriginUpload:
			w.Added.add(size)
		}
	}
	for _, rec := range s.Files {
		arrived(rec.arrival(), rec.Size)
	}
	for _, p := range s.Past {
		arrived(p.Arrived, p.Size)
		w := which(p.GoneAt)
		if w == nil {
			continue
		}
		switch p.Gone {
		case GoneMovedAside:
			w.Archived.add(p.Size)
		case GoneRemoved:
			w.Deleted.add(p.Size)
		case GoneReplaced:
			w.Replaced.add(p.Size)
		case GoneVanished:
			w.Vanished.add(p.Size)
		}
	}
	for _, h := range s.History {
		if w := which(h.Time); w != nil {
			w.Scans++
		}
	}
	return weeks
}

// arrival is how the file came to be here. A download from before v0.8.0
// has no Origin but does have PlacedAt, which only a download ever set.
func (r FileRecord) arrival() Origin {
	if r.Origin.How != "" {
		return r.Origin
	}
	if !r.PlacedAt.IsZero() {
		return Origin{How: OriginDownload, From: r.SourceURL, At: r.PlacedAt}
	}
	return Origin{}
}

// weekStart is midnight on the Monday of t's week, in t's time zone.
func weekStart(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	back := (int(day.Weekday()) + 6) % 7 // Monday is 0, Sunday 6
	return day.AddDate(0, 0, -back)
}
