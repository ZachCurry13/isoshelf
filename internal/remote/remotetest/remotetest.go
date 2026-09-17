// Package remotetest serves recorded responses to tests, so they never reach
// the live network.
//
// Recorded responses live in recorded/<host><path>. A path ending in "/" is
// stored as __index__, and a query string is appended as "@" plus the
// query-escaped query. They were captured from the live sites on 2026-09-17
// and trimmed to what isoshelf reads.
package remotetest

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

//go:embed all:recorded
var recorded embed.FS

// Recorded returns a client transport that answers every request from the
// recorded responses, and 404 for anything not recorded.
func Recorded() http.RoundTripper {
	return Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := fs.ReadFile(recorded, "recorded/"+Name(r.URL))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
}

// MustGet returns the recorded body for rawURL, failing the test if there is
// none.
func MustGet(t testing.TB, rawURL string) []byte {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := fs.ReadFile(recorded, "recorded/"+Name(u))
	if err != nil {
		t.Fatalf("no recorded response for %s", rawURL)
	}
	return body
}

// Name returns the recorded file name for a URL.
func Name(u *url.URL) string {
	name := u.Host + u.Path
	if strings.HasSuffix(name, "/") {
		name += "__index__"
	}
	if u.RawQuery != "" {
		name += "@" + url.QueryEscape(u.RawQuery)
	}
	return name
}

// Handler returns a client transport that passes every request to h, whatever
// its host, without opening a network connection.
func Handler(h http.Handler) http.RoundTripper {
	return roundTripper(func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		// Like a real transport, take the host from the URL when the request
		// doesn't set one (redirects don't).
		serverReq := req.Clone(req.Context())
		if serverReq.Host == "" {
			serverReq.Host = req.URL.Host
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, serverReq)
		resp := rec.Result()
		resp.Request = req
		return resp, nil
	})
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
