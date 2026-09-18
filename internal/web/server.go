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
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

//go:embed static
var staticFiles embed.FS

const (
	cookieName    = "isoshelf_token"
	requestHeader = "X-Isoshelf"
	settingsFile  = "ui.json"
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
	room     space.Usage
	roomAt   time.Time
	roomOf   string
	run      *run
	lastErr  string
	warnings []string
	notice   *appupdate.Notice
}

// run is a scan or check in progress.
type run struct {
	kind     string
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
	mux.HandleFunc("POST /api/scan", func(w http.ResponseWriter, r *http.Request) { s.start(w, false) })
	mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) { s.start(w, true) })
	mux.HandleFunc("POST /api/update", s.startUpdate)
	mux.HandleFunc("POST /api/remove", s.remove)
	mux.HandleFunc("POST /api/removed/empty", s.emptyRemoved)
	mux.HandleFunc("POST /api/cancel", s.cancel)
	mux.HandleFunc("POST /api/track", s.setTrack)
	mux.HandleFunc("GET /api/guesses", s.getGuesses)
	mux.HandleFunc("POST /api/identify", s.identifyFile)
	mux.HandleFunc("POST /api/catalog/refresh", s.updateCatalog)
	mux.HandleFunc("POST /api/settings", s.setSettings)
	mux.HandleFunc("POST /api/catalog/mine", s.addMyImage)
	s.handler = s.guard(mux)

	go s.checkAppUpdate()
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

// stateJSON is everything the page shows.
type stateJSON struct {
	Version   string                 `json:"version"`
	Portable  bool                   `json:"portable"`
	Target    string                 `json:"target"`
	Profile   string                 `json:"profile"`
	UpdatedAt *time.Time             `json:"updated_at,omitempty"`
	Run       *runJSON               `json:"run,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Warnings  []string               `json:"warnings"`
	Report    *check.ReportJSON      `json:"report,omitempty"`
	Tracks    map[string]state.Track `json:"tracks"`
	UsualSet  []string               `json:"usual_set"`
	Recent    []string               `json:"recent_targets"`
	Removed   removedJSON            `json:"removed"`
	Catalog   catalogStatusJSON      `json:"catalog"`
	AppUpdate *appupdate.Notice      `json:"app_update,omitempty"`
	// ReportURL is where a missing image can be reported.
	ReportURL string `json:"report_url,omitempty"`
	// Space is the room left in the folder, when the disk says.
	Space *spaceJSON `json:"space,omitempty"`
}

// spaceJSON is the room left where images are kept.
type spaceJSON struct {
	Free  int64 `json:"free"`
	Total int64 `json:"total"`
}

// removedJSON describes what waits in .isoshelf/removed.
type removedJSON struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

type runJSON struct {
	Kind    string    `json:"kind"`
	Stage   string    `json:"stage"`
	File    string    `json:"file,omitempty"`
	Done    int64     `json:"done"`
	Total   int64     `json:"total"`
	Started time.Time `json:"started"`
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	// Both of these read a disk, so they happen before the lock is taken: a
	// NAS that has gone to sleep must not hold up the whole page.
	recent := s.recentTargets()
	room := s.targetSpace()
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, s.stateLocked(recent, room))
}

// spaceInterval is how long the free space is trusted before asking again.
// The page asks for the state every half second while a scan runs, and on a
// network share every answer costs a round trip.
const spaceInterval = 5 * time.Second

// targetSpace returns the room left in the folder the images are kept in.
// A folder that can't say is not an error worth showing: the page leaves the
// number out instead.
func (s *Server) targetSpace() space.Usage {
	s.mu.Lock()
	target, cached, at, of := s.target, s.room, s.roomAt, s.roomOf
	s.mu.Unlock()
	if target == "" {
		return space.Usage{}
	}
	if of == target && !at.IsZero() && s.cfg.Now().Sub(at) < spaceInterval {
		return cached
	}
	usage, _ := space.Of(target)

	s.mu.Lock()
	s.room, s.roomAt, s.roomOf = usage, s.cfg.Now(), target
	s.mu.Unlock()
	return usage
}

// removedInfo counts what waits in the target's removed folder.
func (s *Server) removedInfo(target string) removedJSON {
	if target == "" {
		return removedJSON{}
	}
	files, bytes, err := update.Removed(target)
	if err != nil {
		return removedJSON{}
	}
	return removedJSON{Files: len(files), Bytes: bytes}
}

// stateLocked builds the page state; s.mu must be held.
func (s *Server) stateLocked(recent []string, room space.Usage) stateJSON {
	out := stateJSON{
		Version:   s.cfg.Version,
		Portable:  s.cfg.Dirs.Portable,
		Target:    s.target,
		Error:     s.lastErr,
		Warnings:  nonNil(s.warnings),
		Tracks:    map[string]state.Track{},
		UsualSet:  []string{},
		Recent:    recent,
		Removed:   s.removedInfo(s.target),
		Catalog:   s.catalogStatusLocked(),
		ReportURL: "https://github.com/" + appupdate.Repo + "/issues/new",
		AppUpdate: s.notice,
	}
	if room.Known() {
		out.Space = &spaceJSON{Free: room.Free, Total: room.Total}
	}
	if s.st != nil {
		out.Profile = string(s.st.Profile)
		out.Tracks = s.st.Tracks
		out.UsualSet = nonNil(s.st.UsualSet())
	}
	if s.report != nil {
		j := s.report.JSON()
		out.Report = &j
		t := s.updatedAt
		out.UpdatedAt = &t
	}
	if s.run != nil {
		out.Run = &runJSON{
			Kind:    s.run.kind,
			Stage:   string(s.run.progress.Stage),
			File:    s.run.progress.File,
			Done:    s.run.progress.Done,
			Total:   s.run.progress.Total,
			Started: s.run.started,
		}
	}
	return out
}

type catalogEntryJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Arch string `json:"arch"`
	// Size is roughly how big the download is, from the catalog.
	Size      int64  `json:"size,omitempty"`
	Updates   string `json:"updates"`
	Page      string `json:"page,omitempty"`
	Site      string `json:"site,omitempty"`
	Forum     string `json:"forum,omitempty"`
	Category  string `json:"category,omitempty"`
	Family    string `json:"family,omitempty"`
	Icon      string `json:"icon,omitempty"`
	IconColor string `json:"icon_color,omitempty"`
	OnTarget  bool   `json:"on_target"`
}

func (s *Server) getCatalog(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	onTarget := map[string]bool{}
	if s.report != nil {
		for _, it := range s.report.Items {
			if it.Entry != nil && it.Path != "" {
				onTarget[it.Entry.ID] = true
			}
		}
	}
	s.mu.Unlock()

	entries := []catalogEntryJSON{}
	cat := s.catalog()
	for i := range cat.Entries {
		e := &cat.Entries[i]
		entries = append(entries, catalogEntryJSON{
			ID: e.ID, Name: e.Name, Arch: e.Arch, Size: e.Size, Updates: e.Updates(), Page: e.Page,
			Site: e.Site, Forum: e.Forum, Category: e.Category, Family: e.Family,
			Icon: e.Icon, IconColor: e.IconColor, OnTarget: onTarget[e.ID],
		})
	}
	slices.SortFunc(entries, func(a, b catalogEntryJSON) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) setTarget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	busy := s.run != nil
	s.mu.Unlock()
	if busy {
		writeError(w, http.StatusConflict, "Wait until the current scan finishes, or stop it.")
		return
	}
	if err := s.openTarget(req.Path, req.Profile); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.getState(w, r)
}

// openTarget makes path the current folder, with profile if given.
func (s *Server) openTarget(path, profile string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("Choose a folder.")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return errors.New("That folder doesn't exist or can't be opened: " + abs)
	}
	st, err := state.Load(abs)
	if err != nil {
		return err
	}
	if profile != "" {
		p, err := scan.ParseProfile(profile)
		if err != nil {
			return err
		}
		st.Profile = p
	}

	s.mu.Lock()
	s.target, s.st, s.report, s.scan, s.lastErr, s.warnings = abs, st, nil, nil, "", nil
	s.mu.Unlock()
	saved := s.loadSettings()
	saved.Target = abs
	s.saveSettings(saved)
	return nil
}

func (s *Server) start(w http.ResponseWriter, online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.target == "":
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.run != nil:
		writeError(w, http.StatusConflict, "A scan is already running.")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	kind := "scan"
	if online {
		kind = "check"
	}
	s.run = &run{kind: kind, started: s.cfg.Now(), progress: inventory.Progress{Stage: inventory.Scanning}, cancel: cancel}
	s.lastErr = ""
	go s.execute(ctx, s.target, s.st.Profile, online)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

func (s *Server) execute(ctx context.Context, target string, profile scan.Profile, online bool) {
	client := s.client()
	res, err := inventory.Run(ctx, inventory.Options{
		Target:  target,
		Profile: profile,
		Online:  online,
		Client:  client,
		Catalog: s.catalog(),
		Dirs:    s.cfg.Dirs,
		Now:     s.cfg.Now,
		Progress: func(p inventory.Progress) {
			s.mu.Lock()
			if s.run != nil {
				s.run.progress = p
			}
			s.mu.Unlock()
		},
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = nil
	if res != nil && s.target == target {
		s.report, s.st, s.scan, s.warnings, s.updatedAt = res.Report, res.State, res.Scan, res.Warnings, s.cfg.Now()
	}
	switch {
	case errors.Is(err, context.Canceled):
		s.lastErr = "Stopped. What was found so far is shown."
	case err != nil:
		s.lastErr = err.Error()
	}
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.run != nil {
		s.run.cancel()
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

func (s *Server) setTrack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entry   string `json:"entry"`
		KeepOld *bool  `json:"keep_old"`
		Starred *bool  `json:"starred"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.run != nil:
		writeError(w, http.StatusConflict, "Wait until the current scan finishes.")
		return
	case s.st == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.cat.Entry(req.Entry) == nil:
		writeError(w, http.StatusBadRequest, "Unknown image.")
		return
	}
	t := s.st.Track(req.Entry)
	if req.KeepOld != nil {
		t.KeepOld = *req.KeepOld
	}
	if req.Starred != nil {
		t.Starred = *req.Starred
	}
	s.st.SetTrack(req.Entry, t)
	if err := s.st.Save(s.target); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the setting: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracks": s.st.Tracks, "usual_set": nonNil(s.st.UsualSet())})
}

// checkAppUpdate looks for a newer isoshelf once, in the background.
func (s *Server) checkAppUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := appupdate.Check(ctx, s.client(), s.cfg.Dirs.Config, s.cfg.Version, s.cfg.Now())
	if err != nil || n == nil {
		return
	}
	s.mu.Lock()
	s.notice = n
	s.mu.Unlock()
}

// recentTargets lists folders checked before, newest first. Portable mode
// keeps no history on the computer.
func (s *Server) recentTargets() []string {
	out := []string{}
	if s.cfg.Dirs.Portable || s.cfg.Dirs.Config == "" {
		return out
	}
	mirrors, err := state.LoadMirrors(s.cfg.Dirs.Config)
	if err != nil {
		return out
	}
	for _, m := range mirrors {
		if m.Path != "" && !slices.Contains(out, m.Path) {
			out = append(out, m.Path)
		}
	}
	return out
}

type settings struct {
	Target string `json:"target,omitempty"`
	// CatalogAuto is nil until the user says either way; the default is on.
	CatalogAuto *bool `json:"catalog_auto,omitempty"`
}

func (s *Server) loadSettings() settings {
	var st settings
	if data, err := os.ReadFile(filepath.Join(s.cfg.Dirs.Config, settingsFile)); err == nil {
		json.Unmarshal(data, &st)
	}
	return st
}

func (s *Server) saveSettings(st settings) {
	if s.cfg.Dirs.Config == "" {
		return
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err == nil && os.MkdirAll(s.cfg.Dirs.Config, 0o755) == nil {
		os.WriteFile(filepath.Join(s.cfg.Dirs.Config, settingsFile), data, 0o644) // best effort
	}
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
