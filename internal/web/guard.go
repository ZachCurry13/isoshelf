package web

// The guard: what every request has to show before isoshelf answers it. The
// rules themselves are in the package comment in server.go.

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// healthPath answers whether isoshelf is up, for a container's health check.
// It is the one path outside the guard, because a health check has no token
// and shouldn't need one - and it says nothing at all: not which folder is
// open, not what is in it, not even the version.
const healthPath = "/healthz"

// loginPath is the username-and-password door. It is outside the guard by
// necessity: somebody who has to log in has not got through it yet.
const loginPath = "/login"

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == healthPath {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte("ok\n"))
		return
	}
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

		if !s.cfg.AnyHost && !isLocalhost(r.Host) {
			http.Error(w, "isoshelf only answers on localhost", http.StatusForbidden)
			return
		}
		// A login replaces the link's secret rather than sitting beside it.
		// Once somebody has set a username and password, that is the way in,
		// and the old link stops working - otherwise the thing it was meant
		// to replace is still there, still in a log, still enough.
		tokenWorks := !s.loginInstead()
		if token := r.URL.Query().Get("token"); tokenWorks && token != "" && r.Method == http.MethodGet && r.URL.Path == "/" {
			if s.validToken(token) {
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		// Three ways in, and they are the same door: the secret in the link,
		// the cookie that link left behind, or a username and password.
		hasToken := false
		if c, err := r.Cookie(cookieName); tokenWorks && err == nil && s.validToken(c.Value) {
			hasToken = true
		}
		if !hasToken && !s.signedIn(r) {
			// The login page is the one thing served to somebody who is not
			// through yet, so it serves itself: no token, no session.
			if r.URL.Path == loginPath && s.canLogIn() {
				s.handleLogin(w, r)
				return
			}
			// A browser asking for a page is sent to the login form. A
			// request from the page's own code is not: it would follow the
			// redirect and get a login page where it expected an answer, so
			// it is refused in the usual way and the page says so.
			if s.canLogIn() && r.Method == http.MethodGet && wantsHTML(r) {
				http.Redirect(w, r, loginPath, http.StatusSeeOther)
				return
			}
			h.Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(s.forbiddenPage()))
			return
		}
		// Already in, so the login page has nothing to say: back to the page.
		// A POST here is somebody signing in again - harmless, and it keeps
		// the meaning of this address to one thing. Signing out is not here:
		// it goes through the ordinary door, where the page's own code can
		// send the header and an Origin that means something.
		if r.URL.Path == loginPath {
			if r.Method == http.MethodPost {
				s.handleLogin(w, r)
				return
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := r.Header.Get("Origin")
			if r.Header.Get(requestHeader) != "1" || !sameOrigin(origin, r.Host) {
				writeError(w, http.StatusForbidden, "request refused")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// wantsHTML says whether this is a browser asking for a page, rather than
// the page's own code asking for an answer.
func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// forbiddenPage is what someone sees who opened the address without the
// secret on the end - which, on a server, is what typing the address into the
// bar does. Where to find that link depends entirely on how isoshelf is
// running, and telling a container user to "go back to the isoshelf window"
// sends them looking for something that doesn't exist.
func (s *Server) forbiddenPage() string {
	where := `<p>For your safety, isoshelf only opens from the link it shows when it
starts. Go back to the isoshelf window and open that link, or start isoshelf
again.</p>`
	if s.cfg.AnyHost {
		where = `<p>isoshelf only opens from the link it prints when it starts, which has a
secret on the end of it. This address on its own isn't enough - that is what
keeps everyone else on the network out.</p>
<p><b>The link is in this container's log.</b> On TrueNAS: Apps, then isoshelf,
then its Logs. With Docker: <code>docker logs isoshelf</code>. Look for a line
beginning <code>http://</code> with <code>?token=</code> in it, and open the
whole thing. Once you have, this address will work on its own.</p>`
	}
	return `<!doctype html><meta charset="utf-8"><title>isoshelf</title>
<body style="font-family:system-ui,sans-serif;max-width:34rem;margin:3rem auto;padding:0 1rem;line-height:1.5">
<h1>Open isoshelf from its link</h1>
` + where
}

func (s *Server) validToken(t string) bool {
	return s.cfg.Token != "" && subtle.ConstantTimeCompare([]byte(t), []byte(s.cfg.Token)) == 1
}

// sameOrigin reports whether Origin, when the browser sent one, names this
// same server. The scheme isn't compared beyond being a web one: behind a
// reverse proxy - which is how this is reached on a NAS - the browser says
// https while the request arrives here as plain http. The host is what has
// to match, and a page on any other site can't forge that.
func sameOrigin(origin, host string) bool {
	if origin == "" {
		return true // not a cross-origin request; the header is absent
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host == host
}

func isLocalhost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
