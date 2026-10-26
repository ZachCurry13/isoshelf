package state

import (
	"slices"
	"strings"
	"time"
)

// What happened in a folder, as a timeline (v0.8.11): the History section.
// It replaced the usage page's week-by-week counts, which the maintainer
// found little use - the useful part was which image came, changed or went,
// and when.
//
// Everything is worked out from records isoshelf already keeps: when each
// file arrived and how (Origin, or PlacedAt for a download from before
// v0.8.0), and when each image left and why (Past). A file that was already
// in the folder when isoshelf first read it has no arrival to show, and an
// image that left before v0.8.6 has lost when it arrived; the timeline shows
// what is left of each story rather than guessing the rest.

// Event kinds. The first three are arrivals and the last four departures;
// an update is an arrival that took the place of an older file of the same
// image, shown once rather than as a file arriving and another leaving.
const (
	EventDownloaded = "downloaded"
	EventCopied     = "copied"
	EventAdded      = "added"
	EventUpdated    = "updated"
	EventArchived   = "archived"
	EventDeleted    = "deleted"
	EventReplaced   = "replaced"
	EventVanished   = "vanished"
)

// Event is one thing that happened to an image in the folder.
type Event struct {
	// At is when; it is zero only for an image that left before isoshelf
	// noted when images left.
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`
	Path string    `json:"path"`
	// Entry and Version say which image and version; From is the version an
	// update replaced, when that is known.
	Entry   string `json:"entry,omitempty"`
	Version string `json:"version,omitempty"`
	From    string `json:"from,omitempty"`
	Size    int64  `json:"size"`
	// Here says the file is in the folder now.
	Here bool `json:"here,omitempty"`
}

// pairWithin is how close in time a file arriving and an older file of the
// same image leaving must be to count as one update. Replacing the old copy
// happens in the same minute; removing it by hand afterwards is still the
// same update if it is done soon after.
const pairWithin = time.Hour

var arrivalKind = map[string]string{
	OriginDownload: EventDownloaded,
	OriginCopy:     EventCopied,
	OriginUpload:   EventAdded,
}

var departureKind = map[string]string{
	GoneMovedAside: EventArchived,
	GoneRemoved:    EventDeleted,
	GoneReplaced:   EventReplaced,
	GoneVanished:   EventVanished,
}

// Events returns what happened from since on, newest first. A zero since
// means everything the records hold.
func (s *State) Events(since time.Time) []Event {
	var arrivals, departures []Event
	arrived := func(o Origin, path, entry, version string, size int64, here bool) {
		kind := arrivalKind[o.How]
		if kind == "" || o.At.IsZero() || o.At.Before(since) {
			return
		}
		e := Event{At: o.At, Kind: kind, Path: path, Entry: entry, Version: version, Size: size, Here: here}
		if o.Replaced != "" {
			e.Kind, e.From = EventUpdated, o.Replaced
		}
		arrivals = append(arrivals, e)
	}
	for path, rec := range s.Files {
		arrived(rec.arrival(), path, rec.Entry, rec.Version, rec.Size, true)
	}
	for _, p := range s.Past {
		arrived(p.Arrived, p.Path, p.Entry, p.Version, p.Size, false)
		kind := departureKind[p.Gone]
		if kind == "" || p.GoneAt.Before(since) || p.GoneAt.IsZero() && !since.IsZero() {
			continue
		}
		departures = append(departures, Event{At: p.GoneAt, Kind: kind, Path: p.Path, Entry: p.Entry, Version: p.Version, Size: p.Size})
	}

	// A file arriving as an older one of the same image leaves is an update.
	// Something vanishing is left alone: it was noticed at a scan, which says
	// nothing about when it actually went.
	for i := range arrivals {
		a := &arrivals[i]
		if a.Entry == "" || a.Kind == EventAdded {
			continue
		}
		j := slices.IndexFunc(departures, func(d Event) bool {
			gap := d.At.Sub(a.At)
			return d.Entry == a.Entry && d.Kind != EventVanished && !d.At.IsZero() &&
				gap > -pairWithin && gap < pairWithin
		})
		if j < 0 {
			continue
		}
		a.Kind = EventUpdated
		if a.From == "" {
			a.From = departures[j].Version
		}
		departures = slices.Delete(departures, j, j+1)
	}

	events := append(arrivals, departures...)
	slices.SortStableFunc(events, func(x, y Event) int {
		if c := y.At.Compare(x.At); c != 0 {
			return c
		}
		return strings.Compare(x.Path, y.Path)
	})
	return events
}
