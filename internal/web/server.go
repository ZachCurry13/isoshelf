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
	"crypto/subtle"
	"embed"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/appupdate"
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

// Config is what the server needs.
type Config struct {
	Dirs    appdir.Dirs
	Catalog *catalog.Catalog
	// HTTP makes requests to download sites; nil means the default client.
	HTTP        *http.Client
	GitHubToken string
	Version     string
	// Token must be presented by every browser.
	Token string
	// Target is the folder to open. Empty means the last one used, or the
	// drive in portable mode.
	Target string
	// CatalogSource says where Catalog came from: "built-in", "downloaded"
	// or "yours". A catalog the user supplied is never replaced.
	CatalogSource string
	// Now defaults to time.Now.
	Now func() time.Time
}

// Server is the web UI. Create it with New.
type Server struct {
	cfg     Config
	handler http.Handler

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
	cancel   context.CancelFunc
}

// New creates the server and starts a background check for a newer isoshelf.
func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	s := &Server{cfg: cfg, cat: cfg.Catalog, catSource: cfg.CatalogSource}
	// What each project said last time. Opening the page then costs nothing
	// for the images already asked about today.
	s.memory = lastcheck.Load(cfg.Dirs.Config)
	s.memory.Now = cfg.Now
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
	mux.HandleFunc("GET /api/guesses", s.getGuesses)
	mux.HandleFunc("POST /api/identify", s.identifyFile)
	mux.HandleFunc("POST /api/catalog/refresh", s.updateCatalog)
	mux.HandleFunc("POST /api/settings", s.setSettings)
	mux.HandleFunc("POST /api/catalog/mine", s.addMyImage)
	mux.HandleFunc("POST /api/bookmark", s.setBookmark)
	s.handler = s.guard(mux)

	if settings.On(s.loadSettings().AppUpdateCheck) {
		go s.checkAppUpdate()
	}
	s.startCatalogRefresh(false)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// guard enforces the localhost, token and same-origin rules.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'")

		if !isLocalhost(r.Host) {
			http.Error(w, "isoshelf only answers on localhost", http.StatusForbidden)
			return
		}
		if token := r.URL.Query().Get("token"); token != "" && r.Method == http.MethodGet && r.URL.Path == "/" {
			if s.validToken(token) {
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		if c, err := r.Cookie(cookieName); err != nil || !s.validToken(c.Value) {
			h.Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(forbiddenPage))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := r.Header.Get("Origin")
			if r.Header.Get(requestHeader) != "1" || (origin != "" && origin != "http://"+r.Host) {
				writeError(w, http.StatusForbidden, "request refused")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

const forbiddenPage = `<!doctype html><meta charset="utf-8"><title>isoshelf</title>
<body>
<h1>Open isoshelf from its link</h1>
<p>For your safety, isoshelf only opens from the link it shows when it starts.
Go back to the isoshelf window and open that link, or start isoshelf again.</p>`

func (s *Server) validToken(t string) bool {
	return s.cfg.Token != "" && subtle.ConstantTimeCompare([]byte(t), []byte(s.cfg.Token)) == 1
}

func isLocalhost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
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
