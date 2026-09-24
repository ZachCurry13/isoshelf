package web

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
)

// isoshelf updating its own program (decisions 13 and 14). Pressing Update
// now downloads the new release, checks that it carries the project's
// signature, waits for any image downloads to finish, puts the new program in
// place and restarts into it on the same port - and the page, which never
// left, reconnects. The command line does the restarting, because only it can
// stop listening and hand over; this file decides when.

// SelfUpdateConfig is what updating isoshelf's own program needs.
type SelfUpdateConfig struct {
	// Exe is the running program.
	Exe string
	// Container is set in the container image, which is updated by pulling
	// a new image, never by replacing the program inside it.
	Container bool
	// Restart hands over to program, just put in place, with the staging
	// folder it confirms or undoes. It returns only if the handover failed.
	Restart func(program, stage string) error
	// From is the version that restarted into this one; Failed says why an
	// update didn't take, when the old program is back.
	From, Failed string
	// Source overrides where releases come from. Tests only.
	Source *appupdate.Source
}

// selfUpdateJSON is what the page shows about it.
type selfUpdateJSON struct {
	// Can says whether Update now is offered; Why says why not.
	Can bool   `json:"can"`
	Why string `json:"why,omitempty"`
	// Stage is "", "downloading", "waiting", "restarting" or "failed".
	Stage   string `json:"stage,omitempty"`
	Done    int64  `json:"done,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Waiting int    `json:"waiting,omitempty"`
	Error   string `json:"error,omitempty"`
	From    string `json:"from,omitempty"`
	Failed  string `json:"failed,omitempty"`
}

// selfUpdate is an update in progress. s.mu guards it.
type selfUpdate struct {
	stage       string
	done, total int64
	err         string
	cancel      context.CancelFunc
}

var releaseVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?$`)

// selfUpdateWhyNot says why this isoshelf can't update itself, or "". Asked
// once at start and again when somebody presses the button: it tries to
// write a file, which is too much to do twice a second.
func (s *Server) selfUpdateWhyNot() string {
	c := s.cfg.SelfUpdate
	switch {
	case !releaseVersion.MatchString(s.cfg.Version):
		return "This is a development build, which doesn't update itself."
	case c.Container:
		return "In a container, isoshelf is updated by pulling the new image."
	case c.Restart == nil || c.Exe == "":
		return "This isoshelf can't restart itself."
	case c.Source == nil && !appupdate.HasKey():
		return appupdate.ErrNoKey.Error()
	}
	if _, err := appupdate.Targets(c.Exe, s.cfg.Dirs.Portable); err != nil {
		return err.Error()
	}
	if err := appupdate.Writable(filepath.Dir(c.Exe)); err != nil {
		return err.Error() + ", so it can't replace itself. Download the new version from the releases page."
	}
	return ""
}

// selfUpdateLocked is the page's view of it. s.mu must be held.
func (s *Server) selfUpdateLocked() selfUpdateJSON {
	u := s.self
	out := selfUpdateJSON{
		Can: s.selfWhyNot == "", Why: s.selfWhyNot,
		Stage: u.stage, Done: u.done, Total: u.total, Error: u.err,
		From: s.cfg.SelfUpdate.From, Failed: s.cfg.SelfUpdate.Failed,
	}
	if u.stage == "waiting" {
		out.Waiting = len(s.queue)
		if s.downloading != nil {
			out.Waiting++
		}
	}
	return out
}

// startSelfUpdate is Update now.
func (s *Server) startSelfUpdate(w http.ResponseWriter, r *http.Request) {
	why := s.selfUpdateWhyNot()
	s.mu.Lock()
	s.selfWhyNot = why
	notice := s.notice
	busy := s.selfBusyLocked()
	if why == "" && notice != nil && !busy {
		ctx, cancel := context.WithCancel(context.Background())
		s.self = selfUpdate{stage: "downloading", cancel: cancel}
		go s.runSelfUpdate(ctx, notice.Latest)
	}
	s.mu.Unlock()
	switch {
	case why != "":
		writeError(w, http.StatusConflict, why)
	case notice == nil:
		writeError(w, http.StatusConflict, "There's no newer isoshelf to update to.")
	case busy:
		writeError(w, http.StatusConflict, "isoshelf is already updating.")
	default:
		s.getState(w, r)
	}
}

// cancelSelfUpdate stops an update that hasn't been put in place yet.
func (s *Server) cancelSelfUpdate(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.self.cancel != nil && s.self.stage != "restarting" {
		s.self.cancel()
	}
	s.mu.Unlock()
	s.getState(w, r)
}

func (s *Server) runSelfUpdate(ctx context.Context, tag string) {
	fail := func(err error) {
		s.mu.Lock()
		s.self = selfUpdate{stage: "failed", err: err.Error()}
		if errors.Is(err, context.Canceled) {
			s.self = selfUpdate{}
		}
		s.mu.Unlock()
	}
	targets, err := appupdate.Targets(s.cfg.SelfUpdate.Exe, s.cfg.Dirs.Portable)
	if err != nil {
		fail(err)
		return
	}
	staged, err := appupdate.Prepare(ctx, s.releaseSource(), tag, targets, func(done, total int64) {
		s.mu.Lock()
		s.self.done, s.self.total = done, total
		s.mu.Unlock()
	})
	if err != nil {
		fail(err)
		return
	}
	// Images first: a restart halfway through a download would throw away
	// the half, and the queue lives only in memory.
	for !s.claimRestart() {
		select {
		case <-ctx.Done():
			fail(ctx.Err())
			return
		case <-time.After(2 * time.Second):
		}
	}
	swapped, err := staged.Swap(s.cfg.Version)
	if err == nil {
		err = s.cfg.SelfUpdate.Restart(swapped.Run, staged.Dir)
		if err != nil {
			swapped.Restore()
		}
	}
	if err != nil {
		fail(errors.New("the new isoshelf couldn't be started, so this one carries on: " + err.Error()))
	}
}

// claimRestart reports whether nothing is running, and if so marks isoshelf
// as restarting so that nothing starts until it has.
func (s *Server) claimRestart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.downloading != nil || len(s.queue) > 0 || s.scanning != nil || s.uploads > 0 {
		s.self.stage = "waiting"
		return false
	}
	s.self.stage = "restarting"
	return true
}

// restartingLocked says whether isoshelf is about to restart into an update,
// when no scan or download may start. s.mu must be held.
func (s *Server) restartingLocked() bool {
	return s.self.stage == "restarting"
}

func (s *Server) releaseSource() appupdate.Source {
	if src := s.cfg.SelfUpdate.Source; src != nil {
		return *src
	}
	fetcher := fetch.New(s.cfg.Version)
	fetcher.HTTP = s.cfg.HTTP
	fetcher.GitHubToken = s.cfg.GitHubToken
	return appupdate.Source{Client: s.client(), Fetch: fetcher}
}

// selfBusyLocked says whether an update is under way, as opposed to not
// started or given up. The scheduler waits for one. s.mu must be held.
func (s *Server) selfBusyLocked() bool {
	return s.self.stage != "" && s.self.stage != "failed"
}
