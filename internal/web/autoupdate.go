package web

import (
	"context"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// Updating the images without being asked.
//
// Turned on, isoshelf checks on a schedule and downloads every update it
// finds, verifies it, and puts it in place - each image following the answer
// it already carries about its old copy (replace, archive, or keep both).
// Nothing about that is new except that nobody pressed anything: it is the
// same queue, the same verification, and the same answers.
//
// It is off unless somebody turns it on, and it always will be. This is the
// one thing isoshelf does that changes a drive while its owner isn't looking,
// and a default that does that is a default that loses somebody a file they
// were relying on.
//
// The rules that hold whether or not anyone is watching:
//   - A checksum that doesn't match still blocks the file. An unverified
//     download still never replaces anything.
//   - Nothing is deleted that the user didn't choose to lose: an image set to
//     archive still archives, and the archive still has to be emptied by hand.
//   - It stops before the folder is full, rather than filling it. A drive with
//     no room left is a drive that can't boot anything either.
//   - A folder that is already busy is left alone. The next run comes round
//     soon enough.

// roomToSpare is how much room isoshelf refuses to use up. Updating a folder
// until the disk is full would be a strange way to look after it, and on a
// Ventoy drive the free space is what the next image goes in.
const roomToSpare = 2 << 30 // 2 GB

// autoUpdateTick is how often the clock is looked at. The schedule itself is
// a day or a week; this is just how promptly isoshelf notices one has come
// round, and how soon after a start it catches up on a missed one.
const autoUpdateTick = 15 * time.Minute

// Run is the part of the server that works while nobody is asking it to: at
// the moment, updating the images on a schedule. It returns when ctx ends.
// The tests don't call it - they call autoUpdateIfDue directly, because a
// test that waits for a real clock is a test nobody runs.
func (s *Server) Run(ctx context.Context) {
	s.watchForUpdates(ctx)
}

// watchForUpdates runs until ctx ends, updating the images when the schedule
// says to.
func (s *Server) watchForUpdates(ctx context.Context) {
	tick := time.NewTicker(autoUpdateTick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.autoUpdateIfDue()
		}
	}
}

// inBackground runs f as work isoshelf started for itself, counted so it can
// be waited for.
func (s *Server) inBackground(f func()) {
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		f()
	}()
}

// autoUpdateIfDue starts a run if one is owed and nothing is in the way.
func (s *Server) autoUpdateIfDue() {
	saved := s.loadSettings()
	if !settings.On(saved.AutoUpdate) || saved.AutoUpdate == nil {
		return
	}
	now := s.cfg.Now()
	last := s.lastAutoUpdate()
	if !last.IsZero() && now.Sub(last) < saved.Every() {
		return
	}

	s.mu.Lock()
	// A folder already being scanned or downloaded into is left alone: the
	// next tick comes round in minutes, and interrupting somebody's own work
	// to do this would be the wrong way round.
	busy := s.target == "" || s.scanning != nil || s.downloading != nil || len(s.queue) > 0 || s.uploads > 0
	s.mu.Unlock()
	if busy {
		return
	}

	s.noteAutoUpdate(now)
	s.mu.Lock()
	// A fresh check first. The queue is filled when that scan ends, from
	// what it found - see scanrun.go.
	s.autoQueue = true
	s.startScanLocked(askAgain)
	s.mu.Unlock()
}

// lastAutoUpdate is when isoshelf last did this by itself. It is kept in the
// settings file, so a restart - which on a NAS is every update - doesn't
// start another run the moment it comes back.
func (s *Server) lastAutoUpdate() time.Time {
	return s.loadSettings().AutoUpdateLast
}

func (s *Server) noteAutoUpdate(now time.Time) {
	s.updateSettings(func(c *settings.Settings) { c.AutoUpdateLast = now.UTC() })
}

// queueUpdatesLocked adds every image with an update waiting to the download
// queue. s.mu must be held. It returns how many it added and why it stopped
// short, if it did.
func (s *Server) queueUpdatesLocked(room space.Usage) (int, string) {
	if s.report == nil || s.st == nil {
		return 0, ""
	}
	free, known := room.Free, room.Known()
	added := 0
	for _, item := range s.report.Items {
		if item.Status != check.UpdateAvailable || item.Entry == nil {
			continue
		}
		if s.queuedLocked(item.Entry.ID) {
			continue
		}
		// Room first. A download that fills the disk helps nobody, and the
		// page's own "will this fit" check says the same thing.
		if known && item.Entry.Size > 0 && free-item.Entry.Size < roomToSpare {
			return added, "stopped before filling the folder"
		}
		if known {
			free -= item.Entry.Size
		}
		var old []string
		for _, other := range s.report.Items {
			if other.Path != "" && other.Entry != nil && other.Entry.ID == item.Entry.ID {
				old = append(old, other.Path)
			}
		}
		s.nextJob++
		s.queue = append(s.queue, &job{
			id: s.nextJob, target: s.target, entry: item.Entry.ID, name: item.Entry.Name,
			size: item.Entry.Size, removal: removalFor(item, s.st.Track(item.Entry.ID), s.loadSettings().OldFiles),
			old: old, automatic: true,
		})
		added++
	}
	if added > 0 {
		s.startNextLocked()
	}
	return added, ""
}

// removalFor is what happens to the copy this update replaces: the image's
// own answer, else the one in Settings, else replace.
//
// The page works the same thing out when it draws the choice (choiceFor in
// details.js). The two have to agree: what happens automatically must be the
// answer the page has been showing the whole time, or somebody's drive does
// something they were told it wouldn't.
func removalFor(item check.Item, track state.Track, fallback string) update.Removal {
	choice := track.OldFiles
	if choice == "" && track.KeepOld {
		choice = settings.OldKeep
	}
	if choice == "" {
		choice = settings.CleanOldFiles(fallback)
	}
	if choice == "" {
		choice = settings.OldReplace
	}
	// An image whose new file has the name the old one already has can't sit
	// beside it, so "keep both" isn't on offer: the page shows archive there,
	// and this follows.
	if choice == settings.OldKeep && sameName(item) {
		choice = settings.OldArchive
	}
	switch choice {
	case settings.OldKeep:
		return update.Keep
	case settings.OldArchive:
		return update.MoveAside
	default:
		return update.DeleteNow
	}
}

// sameName says the update would land on the name the file already has, so
// both copies can't be kept.
func sameName(item check.Item) bool {
	if item.LatestFile == "" || item.Path == "" {
		return false
	}
	return baseName(item.Path) == item.LatestFile
}

func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// autoNote is the one line the page shows about a run nobody asked for, so
// finding four downloads in progress is not a mystery.
func autoNote(added int, stopped string, now time.Time) string {
	note := "isoshelf started " + plural(added, "download") + " by itself"
	if stopped != "" {
		note += ", and " + stopped
	}
	return note + "."
}
