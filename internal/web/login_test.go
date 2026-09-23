package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/auth"
)

// get is a browser asking for a page, which is what decides between being
// sent to the login form and being refused outright.
func get(t *testing.T, s *Server, url string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

// postLogin sends the form the way the page does: the hidden value and the
// cookie it was set with have to agree.
func postLogin(t *testing.T, s *Server, form map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	// Ask for the form first, so its cookie and token are a matching pair.
	page := get(t, s, "http://nas.local:8765/login")
	token := ""
	for _, c := range page.Result().Cookies() {
		if c.Name == formCookie {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("the login form set no cookie, so nothing can be posted back")
	}
	values := make([]string, 0, len(form)+1)
	for k, v := range form {
		values = append(values, k+"="+v)
	}
	values = append(values, formField+"="+token)
	req := httptest.NewRequest(http.MethodPost, "http://nas.local:8765/login", strings.NewReader(strings.Join(values, "&")))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: formCookie, Value: token})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

// On a server with no login set, the address shows the setup form: that is
// the first run people expect, and what the maintainer asked for.
func TestAServerWithNoLoginOffersToSetOneUp(t *testing.T) {
	s := serverMode(t, true)
	rec := get(t, s, "http://nas.local:8765/")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != loginPath {
		t.Fatalf("the address gave %d to %q, want a redirect to %s", rec.Code, rec.Header().Get("Location"), loginPath)
	}
	page := get(t, s, "http://nas.local:8765/login").Body.String()
	for _, want := range []string{"Set up isoshelf", "Password again", "form_token"} {
		if !strings.Contains(page, want) {
			t.Errorf("the setup form doesn't mention %q", want)
		}
	}
}

// A desktop never asks anyone to invent a password: nobody outside the
// machine can reach it, and the browser opens itself with the link.
func TestADesktopIsNotAskedToSetUpALogin(t *testing.T) {
	s := serverMode(t, false)
	rec := get(t, s, "http://127.0.0.1:8765/")
	if rec.Code != http.StatusForbidden {
		t.Errorf("a desktop with no token: %d, want the usual 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Set up isoshelf") {
		t.Error("a desktop was asked to invent a password")
	}
}

// Setting one up gets you straight in, and from then on it is a login form.
func TestSettingUpALoginAndUsingIt(t *testing.T) {
	s := serverMode(t, true)
	rec := postLogin(t, s, map[string]string{
		"user": "zach", "password": "a+good+long+password", "again": "a+good+long+password",
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("setting up: %d %s", rec.Code, rec.Body)
	}
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			session = c
		}
	}
	if session == nil {
		t.Fatal("setting up a login didn't sign anybody in")
	}
	// That session is the way in from now on.
	if rec := get(t, s, "http://nas.local:8765/api/state", session); rec.Code != http.StatusOK {
		t.Errorf("with the session: %d, want 200", rec.Code)
	}
	// And a browser without it gets the login form, not the setup form.
	page := get(t, s, "http://nas.local:8765/login").Body.String()
	if strings.Contains(page, "Set up isoshelf") || strings.Contains(page, "Password again") {
		t.Error("the setup form is still on offer after a login was set")
	}

	// The wrong password is refused, in words, and hands out no session.
	rec = postLogin(t, s, map[string]string{"user": "zach", "password": "not+it"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("a wrong password: %d, want 401", rec.Code)
	}
	// The page is HTML, so an apostrophe arrives as &#39; - match the part
	// of the sentence that survives escaping.
	if !strings.Contains(rec.Body.String(), "That username and password don") {
		t.Errorf("the refusal doesn't say what's wrong: %s", rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			t.Error("a wrong password was given a session anyway")
		}
	}
}

// The two passwords have to be the same, and short ones are refused before
// anything is written.
func TestSettingUpRefusesWhatCannotWork(t *testing.T) {
	for _, c := range []struct{ why, password, again, want string }{
		{"two different passwords", "a+good+long+password", "something+else", "Those two passwords aren"},
		{"a short password", "short", "short", "8 characters"},
	} {
		s := serverMode(t, true)
		rec := postLogin(t, s, map[string]string{"user": "zach", "password": c.password, "again": c.again})
		if !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s: %s, want it to mention %q", c.why, rec.Body, c.want)
		}
		if a, _ := auth.Load(s.cfg.Dirs.Config); a != nil {
			t.Errorf("%s: a login was written anyway", c.why)
		}
	}
}

// The form carries a value only isoshelf could have put there. The usual
// Origin check can't do this job here: isoshelf sends Referrer-Policy:
// no-referrer, and browsers then send "Origin: null" on a plain form post.
func TestALoginFormFromSomewhereElseIsRefused(t *testing.T) {
	s := serverMode(t, true)
	body := "user=zach&password=a+good+long+password&again=a+good+long+password&" + formField + "=guessed"
	req := httptest.NewRequest(http.MethodPost, "http://nas.local:8765/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "null")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a form with a made-up token: %d, want 403", rec.Code)
	}
	if a, _ := auth.Load(s.cfg.Dirs.Config); a != nil {
		t.Error("a form from somewhere else set the password")
	}
}

// The secret in the link stops working once there is a password. Two ways in
// is two ways to get in, and the weaker of the two was printed in a log.
func TestTheLinkStopsWorkingOnceThereIsALogin(t *testing.T) {
	s := serverMode(t, true)
	// Before: the link is how a server is reached.
	if rec := get(t, s, "http://nas.local:8765/api/state", &http.Cookie{Name: cookieName, Value: testToken}); rec.Code != http.StatusOK {
		t.Fatalf("with no login set, the token gave %d, want 200", rec.Code)
	}

	if err := auth.Set(s.cfg.Dirs.Config, "zach", "a good long password"); err != nil {
		t.Fatal(err)
	}

	// After: it is nothing. Not as a cookie somebody still has...
	if rec := get(t, s, "http://nas.local:8765/api/state", &http.Cookie{Name: cookieName, Value: testToken}); rec.Code == http.StatusOK {
		t.Error("a token cookie still got in after a password was set")
	}
	// ...and not in a link, which is the copy that sits in a log forever.
	rec := get(t, s, "http://nas.local:8765/?token="+testToken)
	if rec.Code == http.StatusOK {
		t.Error("the token link still got in after a password was set")
	}
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") != loginPath {
		t.Errorf("the token link went to %q, want the login page", rec.Header().Get("Location"))
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName && c.Value == testToken {
			t.Error("the token link was still handed a cookie")
		}
	}
}

// The page's own requests are refused rather than redirected: following a
// redirect would hand them a login page where an answer was expected.
func TestThePagesOwnRequestsAreRefusedNotRedirected(t *testing.T) {
	s := serverMode(t, true)
	req := httptest.NewRequest(http.MethodGet, "http://nas.local:8765/api/state", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("an unauthenticated api request: %d, want 403", rec.Code)
	}
}

// Guessing is slowed down after a few wrong tries, and the page says how
// long rather than just refusing again.
func TestTooManyWrongPasswordsWait(t *testing.T) {
	s := serverMode(t, true)
	if err := auth.Set(s.cfg.Dirs.Config, "zach", "a good long password"); err != nil {
		t.Fatal(err)
	}
	var last *httptest.ResponseRecorder
	for range tooMany + 1 {
		last = postLogin(t, s, map[string]string{"user": "zach", "password": "wrong"})
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("after %d wrong tries: %d, want 429", tooMany+1, last.Code)
	}
	if !strings.Contains(last.Body.String(), "Too many tries. Wait ") {
		t.Errorf("it doesn't say how long to wait: %s", last.Body)
	}
	// And the right password is made to wait too, or the delay would be
	// trivial to step around.
	rec := postLogin(t, s, map[string]string{"user": "zach", "password": "a+good+long+password"})
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("the right password during a wait: %d, want 429", rec.Code)
	}
}

// The login page carries its own stylesheet inline, which the content policy
// forbids - so the policy for that one page names its hash. Get that wrong
// and the page still works, but arrives as unstyled HTML, which is the kind
// of mistake nobody notices in a test that only reads the body.
func TestTheLoginPageIsAllowedItsOwnStyle(t *testing.T) {
	s := serverMode(t, true)
	rec := get(t, s, "http://nas.local:8765/login")
	policy := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, loginStyleHash) {
		t.Errorf("the policy doesn't allow the page's own style:\n  %s\n  want it to name %s", policy, loginStyleHash)
	}
	// And the hash has to be of what is actually served, not of something
	// the template changed on the way out.
	body := rec.Body.String()
	start := strings.Index(body, "<style>")
	end := strings.Index(body, "</style>")
	if start < 0 || end < start {
		t.Fatal("the login page has no stylesheet at all")
	}
	served := body[start+len("<style>") : end]
	if served != loginStyle {
		t.Error("what the page serves isn't what the hash was taken of, so a browser will refuse it")
	}
	// The rest of the policy still stands.
	for _, want := range []string{"default-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(policy, want) {
			t.Errorf("the policy dropped %q: %s", want, policy)
		}
	}
}

// Nothing on this page may be built from what somebody typed without being
// escaped: it is the one page a stranger can reach.
func TestTheLoginPageEscapesWhatItShows(t *testing.T) {
	s := serverMode(t, true)
	rec := postLogin(t, s, map[string]string{
		"user": "%3Cscript%3Ealert(1)%3C%2Fscript%3E", "password": "short", "again": "short",
	})
	if strings.Contains(rec.Body.String(), "<script>alert(1)</script>") {
		t.Error("the login page put a typed-in script tag straight into the page")
	}
}

// Signing out has to work, and it did not: it was a plain form posted to
// /login, and this server sends Referrer-Policy: no-referrer, so browsers
// send "Origin: null" on a form post and the same-origin check refused every
// sign-out with "request refused". It goes through the ordinary door now,
// where the page's own code sends a real Origin and the header.
func TestSigningOut(t *testing.T) {
	s := serverMode(t, true)
	rec := postLogin(t, s, map[string]string{
		"user": "zach", "password": "a+good+long+password", "again": "a+good+long+password",
	})
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			session = c
		}
	}
	if session == nil {
		t.Fatal("setting up a login didn't sign anybody in")
	}

	// The way the page asks: the change header and an Origin that means
	// something, which is what a fetch sends and a form post cannot.
	req := httptest.NewRequest(http.MethodPost, "http://nas.local:8765/api/login/signout", strings.NewReader("{}"))
	req.AddCookie(session)
	req.Header.Set(requestHeader, "1")
	req.Header.Set("Origin", "http://nas.local:8765")
	out := httptest.NewRecorder()
	s.ServeHTTP(out, req)
	if out.Code != http.StatusOK {
		t.Fatalf("signing out: %d %s", out.Code, out.Body)
	}
	if strings.Contains(out.Body.String(), "request refused") {
		t.Errorf("signing out was refused: %s", out.Body)
	}
	// The cookie is cleared, so the browser holding it is out.
	cleared := false
	for _, c := range out.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("signing out didn't clear the session cookie")
	}
	// And the old session really is no good any more... it is, in fact: the
	// cookie is signed and still valid, which is why signing out is the
	// browser throwing it away. Signing out everywhere is the one that makes
	// an old cookie worthless, and it has its own endpoint.
}

// A POST to /login while already signed in is somebody signing in again, not
// a logout. It used to be read as a logout, which is how signing out came to
// be refused in the first place.
func TestPostingToLoginWhileSignedInIsALogin(t *testing.T) {
	s := serverMode(t, true)
	rec := postLogin(t, s, map[string]string{
		"user": "zach", "password": "a+good+long+password", "again": "a+good+long+password",
	})
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			session = c
		}
	}
	if session == nil {
		t.Fatal("no session to test with")
	}
	// Ask for the form (carrying the session), then post it back.
	page := httptest.NewRequest(http.MethodGet, "http://nas.local:8765/login", nil)
	page.Header.Set("Accept", "text/html")
	page.AddCookie(session)
	pageOut := httptest.NewRecorder()
	s.ServeHTTP(pageOut, page)

	req := httptest.NewRequest(http.MethodPost, "http://nas.local:8765/login",
		strings.NewReader("user=zach&password=a+good+long+password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(session)
	for _, c := range pageOut.Result().Cookies() {
		if c.Name == formCookie {
			req.AddCookie(c)
			req.Body = io.NopCloser(strings.NewReader("user=zach&password=a+good+long+password&" + formField + "=" + c.Value))
		}
	}
	out := httptest.NewRecorder()
	s.ServeHTTP(out, req)
	if strings.Contains(out.Body.String(), "request refused") {
		t.Errorf("signing in while already signed in was refused: %d %s", out.Code, out.Body)
	}
}

// A key that could not be saved means everyone is signed out at every
// restart - on a NAS, every update and every reboot. The page has to say so:
// this was silent, and "why do I have to log in every time?" had no answer on
// the screen. Reported by the maintainer.
func TestThePageSaysWhenLoginsWontSurviveARestart(t *testing.T) {
	dirs := testDirs(t)
	s := newServer(t, dirs, "")
	if err := auth.Set(dirs.Config, "zach", "a good long password"); err != nil {
		t.Fatal(err)
	}

	// Saved: nothing to warn about.
	s.mu.Lock()
	s.keyIsSaved = true
	quiet := s.sessionKeyNote()
	s.mu.Unlock()
	if quiet != "" {
		t.Errorf("a key that saved fine still warns: %s", quiet)
	}

	// Not saved: the page says it, and says where and what to do.
	s.mu.Lock()
	s.keyIsSaved = false
	note := s.sessionKeyNote()
	warnings := s.warningsLocked()
	s.mu.Unlock()
	for _, want := range []string{"signed out", "restarts", dirs.Config, auth.KeyFileName} {
		if !strings.Contains(note, want) {
			t.Errorf("the warning doesn't mention %q:\n  %s", want, note)
		}
	}
	if len(warnings) == 0 || warnings[0] != note {
		t.Errorf("the warning doesn't reach the page's own warnings: %v", warnings)
	}
}

// With nobody logged in there is nobody to sign out, so the warning would be
// noise on a desktop that never asked for a password.
func TestNoLoginMeansNoWarningAboutLosingIt(t *testing.T) {
	s := newServer(t, testDirs(t), "")
	s.mu.Lock()
	s.keyIsSaved = false
	note := s.sessionKeyNote()
	s.mu.Unlock()
	if note != "" {
		t.Errorf("warned about losing a login nobody has set: %s", note)
	}
}
