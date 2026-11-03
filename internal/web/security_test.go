package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/auth"
)

// What a stranger gets from an isoshelf that is open to the network, with a
// login set - which is how it is run behind a tunnel. These are the promises
// that make it safe to put there, as tests, because the way they fail is
// quiet: a new route registered outside the guard works perfectly well for
// everyone, including the people it should refuse.

// networkServerWithLogin is an isoshelf listening on the network, with an
// account set.
func networkServerWithLogin(t *testing.T) *Server {
	t.Helper()
	s := serverMode(t, true)
	if err := auth.Set(s.cfg.Dirs.Config, "owner", "a-long-passphrase"); err != nil {
		t.Fatal(err)
	}
	return s
}

// stranger sends one request from nowhere in particular, with no login unless
// it is given a cookie.
func stranger(s *Server, method, target string, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://isoshelf.example.com"+target, strings.NewReader("{}"))
	req.Header.Set("Accept", "text/html")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

// Every route the server registers, tried with every method and no login,
// answers with a refusal. The routes are read from server.go, so a new one is
// covered the day it is added, whoever adds it.
func TestNoRouteAnswersWithoutALogin(t *testing.T) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := regexp.MustCompile(`mux\.Handle(?:Func)?\("(?:([A-Z]+) )?([^"]+)"`).FindAllStringSubmatch(string(src), -1)
	if len(routes) < 30 {
		t.Fatalf("found %d routes in server.go; this test has lost track of where they are", len(routes))
	}
	s := networkServerWithLogin(t)
	hole := regexp.MustCompile(`\{[^}]*\}`)
	for _, m := range routes {
		path := hole.ReplaceAllString(m[2], "x")
		// The health check and the login form are the doors; both say
		// nothing about the folder.
		if path == healthPath || path == loginPath {
			continue
		}
		for _, method := range []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"} {
			for _, accept := range []string{"text/html", "application/json"} {
				rec := stranger(s, method, path, map[string]string{"Accept": accept, "X-Isoshelf": "1", "Origin": "http://isoshelf.example.com"})
				if rec.Code < 300 {
					t.Errorf("%s %s answered %d to somebody with no login", method, path, rec.Code)
				}
			}
		}
	}
}

// Paths made to look like something else don't get anywhere either.
func TestOddPathsGetNowhereWithoutALogin(t *testing.T) {
	s := networkServerWithLogin(t)
	for _, p := range []string{"/../../etc/passwd", "//", "/%2e%2e/%2e%2e/etc/passwd", "/static/../static/app.js", "/static/app.js", "/static/css/tokens.css", "/logo/debian.svg", "/api/state", "/nothing"} {
		if rec := stranger(s, "GET", p, map[string]string{"Accept": "application/json"}); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s answered %d, want 403", p, rec.Code)
		}
	}
}

// A session cookie is worth something only if isoshelf signed it, for a login
// that has not expired.
func TestForgedAndStaleSessionsAreRefused(t *testing.T) {
	s := networkServerWithLogin(t)
	good := s.mintSession("owner")
	if rec := stranger(s, "GET", "/api/state", nil, &http.Cookie{Name: sessionCookie, Value: good}); rec.Code != http.StatusOK {
		t.Fatalf("a real session was refused: %d", rec.Code)
	}
	for name, value := range map[string]string{
		"tampered":        strings.Replace(good, "|", "x|", 1),
		"unsigned":        good[:strings.LastIndex(good, "|")],
		"empty":           "",
		"another key":     auth.Key([]byte("0123456789abcdef0123456789abcdef")).Mint("owner", time.Now()),
		"long since gone": s.sessions.Mint("owner", time.Now().Add(-40*24*time.Hour)),
	} {
		if rec := stranger(s, "GET", "/api/state", map[string]string{"Accept": "application/json"}, &http.Cookie{Name: sessionCookie, Value: value}); rec.Code != http.StatusForbidden {
			t.Errorf("a %s session got %d, want 403", name, rec.Code)
		}
	}
}

// The link's secret is a way in only until a login exists.
func TestTheLinkIsDeadOnceThereIsALogin(t *testing.T) {
	s := networkServerWithLogin(t)
	rec := stranger(s, "GET", "/?token="+testToken, nil)
	if len(rec.Result().Cookies()) > 0 {
		t.Error("the link still handed out a cookie after a login was set")
	}
	if rec := stranger(s, "GET", "/api/state", map[string]string{"Accept": "application/json"}, &http.Cookie{Name: cookieName, Value: testToken}); rec.Code != http.StatusForbidden {
		t.Errorf("the link's cookie got %d after a login was set, want 403", rec.Code)
	}
}

// Nobody who isn't the owner can write the first password, with or without a
// form of their own making.
func TestAStrangerCannotChooseTheFirstPassword(t *testing.T) {
	s := serverMode(t, true)
	req := httptest.NewRequest(http.MethodPost, "http://isoshelf.example.com/login",
		strings.NewReader("user=mallory&password=a-long-passphrase&again=a-long-passphrase&"+formField+"=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: formCookie, Value: "x"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code < 400 {
		t.Errorf("a stranger's setup form got %d, want a refusal", rec.Code)
	}
	if a, _ := auth.Load(s.cfg.Dirs.Config); a != nil {
		t.Fatalf("a stranger set the login to %q", a.User)
	}
}

// Behind a tunnel the browser speaks https and the request arrives as plain
// http with the proxy's word for it, so that word is what marks the cookie
// Secure. Without it the cookie is also sent if the same name is ever opened
// over http.
func TestTheCookiesAreSecureWhenTheVisitorIsOnHTTPS(t *testing.T) {
	for name, c := range map[string]struct {
		headers map[string]string
		secure  bool
	}{
		"X-Forwarded-Proto": {map[string]string{"X-Forwarded-Proto": "https"}, true},
		"a list of them":    {map[string]string{"X-Forwarded-Proto": "https, http"}, true},
		"Cloudflare":        {map[string]string{"CF-Visitor": `{"scheme":"https"}`}, true},
		"plain http":        {nil, false},
		"http said so":      {map[string]string{"X-Forwarded-Proto": "http"}, false},
	} {
		s := networkServerWithLogin(t)
		form := stranger(s, "GET", "/login", c.headers)
		var token string
		for _, ck := range form.Result().Cookies() {
			if ck.Name == formCookie {
				token = ck.Value
				if ck.Secure != c.secure {
					t.Errorf("%s: the form cookie's Secure is %v, want %v", name, ck.Secure, c.secure)
				}
			}
		}
		req := httptest.NewRequest(http.MethodPost, "http://isoshelf.example.com/login",
			strings.NewReader("user=owner&password=a-long-passphrase&"+formField+"="+token))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: formCookie, Value: token})
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		var session *http.Cookie
		for _, ck := range rec.Result().Cookies() {
			if ck.Name == sessionCookie {
				session = ck
			}
		}
		if session == nil {
			t.Fatalf("%s: logging in set no session", name)
		}
		if session.Secure != c.secure || !session.HttpOnly || session.SameSite != http.SameSiteStrictMode {
			t.Errorf("%s: session cookie Secure=%v HttpOnly=%v SameSite=%v, want Secure=%v, HttpOnly and SameSite=Strict",
				name, session.Secure, session.HttpOnly, session.SameSite, c.secure)
		}
	}
}

// guess posts one login attempt and says what came back.
func guess(s *Server, password string, headers map[string]string) int {
	form := stranger(s, "GET", "/login", nil)
	var token string
	for _, c := range form.Result().Cookies() {
		if c.Name == formCookie {
			token = c.Value
		}
	}
	req := httptest.NewRequest(http.MethodPost, "http://isoshelf.example.com/login",
		strings.NewReader("user=owner&password="+password+"&"+formField+"="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: formCookie, Value: token})
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec.Code
}

// Without the setting, a header naming another visitor changes nothing: the
// guesser can't escape the wait by writing a new address with every guess.
func TestAGuesserCannotShakeOffTheWaitByNamingAnotherAddress(t *testing.T) {
	s := networkServerWithLogin(t)
	for i := 0; i < tooMany; i++ {
		guess(s, "wrong", map[string]string{"CF-Connecting-IP": "203.0.113." + string(rune('1'+i))})
	}
	if code := guess(s, "wrong", map[string]string{"CF-Connecting-IP": "198.51.100.77", "X-Forwarded-For": "198.51.100.78"}); code != http.StatusTooManyRequests {
		t.Errorf("a guess under a fresh address got %d, want 429", code)
	}
}

// Told it sits behind a trusted proxy, isoshelf keeps visitors apart: a
// stranger's wrong guesses make the stranger wait, and the owner does not.
func TestBehindATrustedProxyAStrangersGuessesDoNotLockOutTheOwner(t *testing.T) {
	s := networkServerWithLogin(t)
	s.cfg.TrustProxy = true
	for i := 0; i < tooMany+2; i++ {
		guess(s, "wrong", map[string]string{"CF-Connecting-IP": "203.0.113.9"})
	}
	if code := guess(s, "wrong", map[string]string{"CF-Connecting-IP": "203.0.113.9"}); code != http.StatusTooManyRequests {
		t.Fatalf("the stranger's next guess got %d, want 429", code)
	}
	if code := guess(s, "a-long-passphrase", map[string]string{"CF-Connecting-IP": "198.51.100.1"}); code != http.StatusSeeOther {
		t.Errorf("the owner, from another address, got %d, want to be let in", code)
	}
	// With no header from the proxy of its own, it is the last entry of
	// X-Forwarded-For that counts, the one the nearest proxy added; the ones
	// before it are whatever the visitor chose to write.
	if code := guess(s, "wrong", map[string]string{"X-Forwarded-For": "198.51.100.200, 203.0.113.9"}); code != http.StatusTooManyRequests {
		t.Errorf("a guess with a made-up first X-Forwarded-For entry got %d, want 429 as the proxy's entry is the same", code)
	}
	// Something that isn't an address is not used.
	if got := proxiedAddr(httptest.NewRequest("GET", "/", nil)); got != "" {
		t.Errorf("with no header the proxied address is %q", got)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("CF-Connecting-IP", "not an address")
	if got := proxiedAddr(r); got != "" {
		t.Errorf("a made-up address was used: %q", got)
	}
}
