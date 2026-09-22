package web

import (
	"crypto/rand"
	"crypto/subtle"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/auth"
)

// A username and password, for isoshelf on a network.
//
// The secret in the link still works and is still how a desktop opens
// itself. This is the thing people expect from something running on their
// NAS: a login box, set once, used from every device, with nothing to find in
// a log afterwards.
//
// Nobody has one until somebody sets one. Until then, a server shows the
// setup form to whoever opens it - which is how every NAS app does its first
// run, and which is worth being plain about: on first start, the first person
// to reach the address chooses the password. isoshelf says so in its log,
// with the address, so it is not a surprise. Setting ISOSHELF_USERNAME and
// ISOSHELF_PASSWORD before it ever starts skips that window entirely.
//
// The link's secret keeps working after a password is set. It is the way back
// in for somebody who has forgotten theirs, and it is no weaker than the
// password: both are in reach of anyone who can read the container's log or
// its config folder.

const (
	sessionCookie = "isoshelf_session"
	// formCookie holds a value the login form echoes back, so a form on
	// somebody else's site can't post here.
	//
	// The usual check - is the Origin header this same server? - cannot work
	// on this one form. isoshelf sends Referrer-Policy: no-referrer, and for
	// a plain form submission browsers then send "Origin: null", by the
	// letter of the spec. Every other change isoshelf makes goes through the
	// page's own code, where the Origin is real; this form has to work with
	// no JavaScript at all, so it carries its own proof instead: a random
	// value set in a cookie and written into the form. A form on another
	// site can produce neither, because the cookie is SameSite=Strict and
	// nobody else can read it to copy it into their own form.
	formCookie = "isoshelf_form"
	formField  = "form_token"
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
	if f.count >= tooMany {
		wait := firstWait << min(f.count-tooMany, 6)
		f.until = now.Add(min(wait, longestWait))
	}
	// Addresses that gave up long ago are forgotten, so this map can't grow
	// without end on a network where something keeps knocking.
	for other, old := range a.failed {
		if other != addr && old.count < tooMany && now.Sub(old.until) > time.Hour {
			delete(a.failed, other)
		}
	}
}

func (a *attempts) success(addr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.failed, addr)
}

// addrOf is who is knocking, for the lockout. It is the connection's own
// address: isoshelf is not behind a load balancer it controls, so a header
// saying otherwise is somebody's claim rather than a fact.
func addrOf(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// account is the login as it is on disk right now, or nil if nobody has set
// one. It is read each time rather than remembered, so a password changed on
// the command line takes effect without a restart.
func (s *Server) account() *auth.Account {
	a, err := auth.Load(s.cfg.Dirs.Config)
	if err != nil {
		return nil
	}
	return a
}

// loginInstead says the link's secret no longer works, because a username and
// password have taken its place.
//
// The maintainer asked for this outright: "I want to get rid of tokens moving
// forward and only have a login screen." Two ways in is two ways to get in,
// and the weaker one was the one printed in a log that anybody with access to
// the machine can read. So the link is how isoshelf opens on a desktop, and
// how a server is reached until somebody sets a password - and after that it
// is nothing.
//
// Forgetting the password is therefore not a lock-out but it is not the page
// either: ISOSHELF_USERNAME and ISOSHELF_PASSWORD set a new one at the next
// start, and "isoshelf password" sets one from a shell. Both need the machine
// itself, which is the right bar for getting back in.
func (s *Server) loginInstead() bool {
	return s.account() != nil
}

// canLogIn says whether the username-and-password door is open at all: once
// somebody has set a login, and on a server that hasn't got one yet, where
// the first person to arrive is the one who sets it.
//
// A desktop with no login set never shows the setup form. There the browser
// opens itself with the link, nobody outside the machine can reach it, and a
// form asking to invent a password would be a question with no purpose.
func (s *Server) canLogIn() bool {
	return s.account() != nil || s.cfg.AnyHost
}

// signedIn says whether this request carries a session isoshelf signed.
func (s *Server) signedIn(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	// The key can be replaced while isoshelf runs - that is what signing out
	// everywhere does - so it is read under the lock like anything else.
	s.mu.Lock()
	key := s.sessions
	s.mu.Unlock()
	return key != nil && key.Valid(c.Value, s.cfg.Now())
}

// mintSession signs a cookie value for user, under the key as it is now.
func (s *Server) mintSession(user string) string {
	s.mu.Lock()
	key := s.sessions
	s.mu.Unlock()
	if key == nil {
		return ""
	}
	return key.Mint(user, s.cfg.Now())
}

// setSession puts the cookie on, or takes it off when user is empty.
func (s *Server) setSession(w http.ResponseWriter, user string) {
	c := &http.Cookie{
		Name: sessionCookie, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	if user == "" {
		c.MaxAge = -1
	} else {
		c.Value = s.mintSession(user)
		c.MaxAge = int(auth.SessionLife / time.Second)
	}
	http.SetCookie(w, c)
}

// formToken sets the value this form and its cookie must agree on, and
// returns it for the page to write into the form.
func (s *Server) formToken(w http.ResponseWriter) string {
	value := rand.Text()
	http.SetCookie(w, &http.Cookie{
		Name: formCookie, Value: value, Path: loginPath, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: int(time.Hour / time.Second),
	})
	return value
}

// formIsOurs says whether this POST came from a form isoshelf drew.
func formIsOurs(r *http.Request) bool {
	c, err := r.Cookie(formCookie)
	if err != nil || c.Value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(r.PostFormValue(formField))) == 1
}

// handleLogin answers the login and setup forms. It runs before the guard,
// because somebody who has to log in is by definition not through it yet.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	have := s.account()
	if r.Method == http.MethodGet {
		s.writeLoginPage(w, have, "", http.StatusOK)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.writeLoginPage(w, have, "That didn't arrive properly. Try again.", http.StatusBadRequest)
		return
	}
	// A form on somebody else's site can produce neither half of this.
	if !formIsOurs(r) {
		s.writeLoginPage(w, have, "That form had gone stale. Try again.", http.StatusForbidden)
		return
	}
	user, password := r.PostFormValue("user"), r.PostFormValue("password")

	if have == nil {
		// Nobody has a login yet: this is the one time the form sets one.
		if again := r.PostFormValue("again"); again != password {
			s.writeLoginPage(w, nil, "Those two passwords aren't the same.", http.StatusOK)
			return
		}
		if err := auth.Set(s.cfg.Dirs.Config, user, password); err != nil {
			s.writeLoginPage(w, nil, err.Error(), http.StatusOK)
			return
		}
		s.setSession(w, user)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	who := addrOf(r)
	if wait := s.logins.wait(who, s.cfg.Now()); wait > 0 {
		s.writeLoginPage(w, have, "Too many tries. Wait "+plainDuration(wait)+" and try again.", http.StatusTooManyRequests)
		return
	}
	if !have.Matches(user, password) {
		s.logins.failure(who, s.cfg.Now())
		s.writeLoginPage(w, have, "That username and password don't match.", http.StatusUnauthorized)
		return
	}
	s.logins.success(who)
	s.setSession(w, have.User)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout ends this browser's session. Everyone else stays logged in;
// signing out everywhere is in Settings, and throws the signing key away.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r.Header.Get("Origin"), r.Host) {
		writeError(w, http.StatusForbidden, "request refused")
		return
	}
	s.setSession(w, "")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// plainDuration is "20 seconds" or "3 minutes", for somebody to read.
func plainDuration(d time.Duration) string {
	if d < time.Minute {
		return plural(int(d.Round(time.Second)/time.Second), "second")
	}
	return plural(int(d.Round(time.Minute)/time.Minute), "minute")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
