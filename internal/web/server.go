// Package web serves isoshelf's user interface: one page plus a small JSON
// API over the same core packages the CLI uses. It is the only package that
// knows about HTTP handlers and HTML.
//
// The server is meant to listen on localhost only. Every browser must bring
// the random token from the link isoshelf prints (it's then kept in a
// cookie), requests must name a localhost host (which blocks DNS
// rebinding), and changes need a custom header that other websites can't
// send.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/auth"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/lastcheck"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

//go:embed static
var staticFiles embed.FS

const (
	cookieName    = "isoshelf_token"
	requestHeader = "X-Isoshelf"
)

// Server is the web UI. Create it with New.
type Server struct {
	cfg     Config
	handler http.Handler
	// sessions signs the cookie a browser holds after someone logs in, and
	// logins is how a wrong password slows the next guess down.
	sessions auth.Key
	logins   *attempts
	// keyIsSaved is false when the signing key exists only in memory, because
	// isoshelf could not write it to its own folder. Everyone is then signed
	// out at every restart, and the page has to say so - see sessionKeyNote.
	keyIsSaved bool
	// background counts the work isoshelf starts for itself - a scheduled
	// update, so far. Counting it means a test can wait for it instead of
	// racing its own cleanup, and means there is something to wait on the
	// day this needs a tidy shutdown.
	background sync.WaitGroup
	// settingsMu holds the settings file still for a read-modify-write.
	// Kept apart from mu, which guards the folder and the queue: a settings
	// write reads a disk, and nothing about the page should wait for it.
	settingsMu sync.Mutex

	mu     sync.Mutex
	target string
	st     *state.State
	// cat is the catalog in use. A newer published one can replace it while
	// isoshelf runs, so it is read through s.catalog() or under the lock.
	cat       *catalog.Catalog
	catSource string
	catNote   string
	catErr    string
	// scan is the last look at the folder, kept so a file can be identified
	// without reading the disk again.
	scan      *scan.Result
	report    *check.Report
	updatedAt time.Time
	// room is the free space where images are kept, and roomAt when it was
	// last asked for, of the folder roomOf.
	room   space.Usage
	roomAt time.Time
	roomOf string
	// A scan and a download run side by side, so they have a slot each: the
	// page is never locked for the length of a queue. Only one scan runs at
	// a time, and only one download.
	scanning    *run
	downloading *run
	// autoQueue marks the scan that the scheduler started, so that when it
	// finishes its findings go straight into the download queue. autoNote is
	// the line the page shows about what it did.
	autoQueue bool
	// autoOffset is added to the schedule, a different amount in each
	// isoshelf, so copies that started together don't all ask the same
	// servers in the same minute every day (#59).
	autoOffset time.Duration
	autoNote   string
	// queue holds the downloads waiting their turn, in order; finished the
	// ones that ended, newest first. placed says a download has put a file in
	// the folder since the last scan.
	queue    []*job
	finished []finishedJob
	nextJob  int
	placed   bool
	// uploads is how many files are arriving from the user's computer right
	// now. They write straight to the folder, so switching folders waits for
	// them, but nothing else does.
	uploads int
	// runJob downloads one queued image. Tests replace it.
	runJob   func(context.Context, *job) (note string, err error)
	lastErr  string
	warnings []string
	notice   *appupdate.Notice
	// self is isoshelf updating its own program, and selfWhyNot why it
	// can't, worked out at start and whenever somebody presses the button.
	self       selfUpdate
	selfWhyNot string
	// memory is what each image's project said last time isoshelf asked. It
	// has its own lock, so it is read and written without holding s.mu.
	memory *lastcheck.Answers
}

// run is a scan, check or download in progress.
type run struct {
	kind string
	// job is the download, when it is one: a run in s.downloading always has
	// one, a run in s.scanning never does.
	job      *job
	started  time.Time
	progress inventory.Progress
	// from is where the bytes are arriving from, in words: the name of the
	// other isoshelf, or "the internet". Copying from a peer is the whole
	// point of setting one up and was invisible while it happened.
	from   string
	cancel context.CancelFunc
}

// New creates the server and starts a background check for a newer isoshelf.
func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	s := &Server{cfg: cfg, cat: cfg.Catalog, catSource: cfg.CatalogSource, logins: newAttempts()}
	// A key isoshelf can't save still works; it just means everyone has to
	// log in again after a restart, which is better than refusing to start.
	// Whether it saved is remembered, because a person who has to log in
	// every time deserves to be told why rather than left guessing.
	var savedKey bool
	s.sessions, savedKey, _ = auth.LoadKey(cfg.Dirs.Config)
	s.keyIsSaved = savedKey
	// What each project said last time. Opening the page then costs nothing
	// for the images already asked about today.
	s.memory = lastcheck.Load(cfg.Dirs.Config)
	s.memory.Now = cfg.Now
	s.selfWhyNot = s.selfUpdateWhyNot()
	s.autoOffset = rand.N(time.Hour)
	s.runJob = s.runUpdate
	if s.catSource == "" {
		s.catSource = catalogBuiltIn
	}

	target := cfg.Target
	if target == "" {
		target = s.loadSettings().Target
	}
	if target == "" && cfg.Dirs.Portable {
		target = cfg.Dirs.DefaultTarget
	}
	if target != "" {
		s.openTarget(target, "") // a folder that's gone just means choosing again
	}

	static, _ := fs.Sub(staticFiles, "static")
	mux := http.NewServeMux()
	mux.Handle("GET /{$}", http.FileServerFS(static))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /api/state", s.getState)
	mux.HandleFunc("GET /api/catalog", s.getCatalog)
	mux.HandleFunc("GET /api/browse", s.browse)
	mux.HandleFunc("GET /logo/{slug}", s.logo)
	mux.HandleFunc("GET /api/archive", s.getArchive)
	mux.HandleFunc("POST /api/restore", s.restore)
	mux.HandleFunc("POST /api/target", s.setTarget)
	// A scan checks for updates too, unless Settings says not to; Refresh
	// asks every project again however recently it was asked.
	mux.HandleFunc("POST /api/scan", func(w http.ResponseWriter, r *http.Request) { s.start(w, askIfDue) })
	mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) { s.start(w, askAgain) })
	mux.HandleFunc("POST /api/update", s.startUpdate)
	mux.HandleFunc("POST /api/queue/move", s.moveQueued)
	mux.HandleFunc("POST /api/queue/drop", s.dropQueued)
	mux.HandleFunc("POST /api/queue/clear", s.clearFinished)
	mux.HandleFunc("POST /api/upload", s.uploadFile)
	mux.HandleFunc("POST /api/remove", s.remove)
	mux.HandleFunc("POST /api/removed/empty", s.emptyRemoved)
	mux.HandleFunc("POST /api/cancel", s.cancel)
	mux.HandleFunc("POST /api/track", s.setTrack)
	mux.HandleFunc("GET /api/report", s.getReport)
	mux.HandleFunc("GET /api/guesses", s.getGuesses)
	mux.HandleFunc("POST /api/identify", s.identifyFile)
	mux.HandleFunc("POST /api/catalog/refresh", s.updateCatalog)
	mux.HandleFunc("POST /api/settings", s.setSettings)
	mux.HandleFunc("POST /api/records", s.setRecords)
	mux.HandleFunc("POST /api/records/plan", s.planRecords)
	mux.HandleFunc("POST /api/selfupdate", s.startSelfUpdate)
	mux.HandleFunc("POST /api/selfupdate/cancel", s.cancelSelfUpdate)
	mux.HandleFunc("POST /api/folders/forget", s.forgetFolder)
	mux.HandleFunc("POST /api/login", s.setLogin)
	mux.HandleFunc("POST /api/login/everywhere", s.signOutEverywhere)
	mux.HandleFunc("POST /api/login/signout", s.signOut)
	mux.HandleFunc("POST /api/peer", s.setPeer)
	mux.HandleFunc("POST /api/share", s.setSharing)
	mux.HandleFunc("GET /api/share/have", s.shareHave)
	mux.HandleFunc("GET /api/share/file/{name}", s.shareFile)
	mux.HandleFunc("POST /api/catalog/mine", s.addMyImage)
	mux.HandleFunc("POST /api/bookmark", s.setBookmark)
	s.handler = s.guard(mux)

	if settings.On(s.loadSettings().AppUpdateCheck) {
		go s.checkAppUpdate()
	}
	s.startCatalogRefresh(false)
	return s
}

// nonNil returns s, or an empty slice so JSON shows [] instead of null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
