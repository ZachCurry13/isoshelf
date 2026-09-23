package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

// serverMode is a server told it may be reached by the machine's own address,
// the way it is when it runs in a container.
func serverMode(t *testing.T, anyHost bool) *Server {
	t.Helper()
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{
		Dirs:    testDirs(t),
		Catalog: cat,
		HTTP:    &http.Client{Transport: remotetest.Recorded()},
		Version: "dev",
		Token:   testToken,
		AnyHost: anyHost,
	})
}

func ask(t *testing.T, s *Server, method, url string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

// By default the page answers only to localhost, whatever token is presented.
func TestOnlyLocalhostByDefault(t *testing.T) {
	s := serverMode(t, false)
	rec := ask(t, s, http.MethodGet, "http://nas.local:8765/api/state", nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a name that isn't localhost: %d, want 403", rec.Code)
	}
	if rec := ask(t, s, http.MethodGet, "http://127.0.0.1:8765/api/state", nil); rec.Code != http.StatusOK {
		t.Errorf("localhost: %d, want 200", rec.Code)
	}
}

// Told it may be, it answers to the machine's own name - and the token is
// still the thing that decides, not the address.
func TestServerModeStillNeedsTheToken(t *testing.T) {
	s := serverMode(t, true)
	if rec := ask(t, s, http.MethodGet, "http://nas.local:8765/api/state", nil); rec.Code != http.StatusOK {
		t.Errorf("with the token: %d, want 200", rec.Code)
	}

	// No cookie at all: refused, however right the address is.
	req := httptest.NewRequest(http.MethodGet, "http://nas.local:8765/api/state", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("without the token: %d, want 403", rec.Code)
	}

	// A wrong token is no better.
	req = httptest.NewRequest(http.MethodGet, "http://nas.local:8765/api/state", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "not-the-token-at-all"})
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("with a wrong token: %d, want 403", rec.Code)
	}
}

// Changes still need the header no other site can send, and an Origin that
// names this same server. Server mode relaxes the address, nothing else.
func TestServerModeKeepsTheCrossSiteRules(t *testing.T) {
	s := serverMode(t, true)
	const url = "http://nas.local:8765/api/scan"

	if rec := ask(t, s, http.MethodPost, url, nil); rec.Code != http.StatusForbidden {
		t.Errorf("a change without the header: %d, want 403", rec.Code)
	}
	bad := map[string]string{requestHeader: "1", "Origin": "http://evil.example"}
	if rec := ask(t, s, http.MethodPost, url, bad); rec.Code != http.StatusForbidden {
		t.Errorf("a change from another site: %d, want 403", rec.Code)
	}
	// Its own origin is fine, and so is https, because a reverse proxy in
	// front of this speaks https to the browser and http to us.
	for _, origin := range []string{"http://nas.local:8765", "https://nas.local:8765"} {
		ok := map[string]string{requestHeader: "1", "Origin": origin}
		if rec := ask(t, s, http.MethodPost, url, ok); rec.Code == http.StatusForbidden {
			t.Errorf("a change from %s was refused", origin)
		}
	}
}

func TestSameOrigin(t *testing.T) {
	host := "nas.local:8765"
	for _, c := range []struct {
		origin string
		want   bool
	}{
		{"", true}, // no Origin: not a cross-origin request
		{"http://nas.local:8765", true},
		{"https://nas.local:8765", true},
		{"http://nas.local", false},      // a different port is a different origin
		{"http://evil.example", false},   // plainly somebody else
		{"file://nas.local:8765", false}, // not a web origin
		{"://nope", false},
	} {
		if got := sameOrigin(c.origin, host); got != c.want {
			t.Errorf("sameOrigin(%q, %q) = %v, want %v", c.origin, host, got, c.want)
		}
	}
}

// The health check needs no token, and gives nothing away.
func TestHealthNeedsNoToken(t *testing.T) {
	s := serverMode(t, true)
	req := httptest.NewRequest(http.MethodGet, "http://nas.local:8765/healthz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Errorf("health: %d %q", rec.Code, rec.Body)
	}
	// It must not become a way to read anything.
	for _, leak := range []string{"token", testToken, "folder", "isoshelf-"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("the health check mentions %q: %q", leak, rec.Body)
		}
	}
}

// Someone who opens the bare address gets told where to find the link - and
// on a server that is the container's log, not an "isoshelf window" that
// doesn't exist there.
func TestForbiddenPageSaysWhereTheLinkIs(t *testing.T) {
	desktop := serverMode(t, false).forbiddenPage()
	if !strings.Contains(desktop, "isoshelf window") {
		t.Errorf("on a desktop it should point at the window: %q", desktop)
	}
	if strings.Contains(desktop, "docker logs") {
		t.Error("a desktop was told to look in a container log")
	}

	server := serverMode(t, true).forbiddenPage()
	for _, want := range []string{"log", "docker logs isoshelf", "?token=", "TrueNAS"} {
		if !strings.Contains(server, want) {
			t.Errorf("a server's page doesn't mention %q: %q", want, server)
		}
	}
	if strings.Contains(server, "isoshelf window") {
		t.Error("a container user was sent looking for a window that doesn't exist")
	}
	// It must never be the place the secret leaks.
	if strings.Contains(server, testToken) {
		t.Error("the refusal page contains the token")
	}
}

// Somebody running one isoshelf on their desktop and another on their NAS has
// two browser tabs that would otherwise be called the same thing. The one
// reachable from other machines says so - on the login page, which is the
// first thing a server shows, and in the state the page names its tab from.
func TestTheServerSaysSoInTheTab(t *testing.T) {
	server, desktop := serverMode(t, true), serverMode(t, false)
	if got, want := server.pageName(), "isoshelf server"; got != want {
		t.Errorf("server tab is %q, want %q", got, want)
	}
	if got, want := desktop.pageName(), "isoshelf"; got != want {
		t.Errorf("desktop tab is %q, want %q", got, want)
	}

	rec := httptest.NewRecorder()
	server.writeLoginPage(rec, nil, "", http.StatusOK)
	if !strings.Contains(rec.Body.String(), "<title>Set up isoshelf server</title>") {
		t.Error("the setup page a server shows doesn't name itself a server")
	}

	if body := ask(t, server, http.MethodGet, "/api/state", nil).Body.String(); !strings.Contains(body, `"server":true`) {
		t.Error("the page state doesn't say this is a server")
	}
	// A desktop answers only to localhost, so ask it there.
	if body := ask(t, desktop, http.MethodGet, "http://127.0.0.1/api/state", nil).Body.String(); !strings.Contains(body, `"server":false`) {
		t.Error("a desktop isoshelf claims to be a server")
	}
}
