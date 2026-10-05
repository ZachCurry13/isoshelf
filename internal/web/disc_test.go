package web

import (
	"net/http"
	"strings"
	"testing"
)

// The plain disc (TODO item 9) ships with isoshelf, for images whose
// project's logo can't be shown here, and the logos taken out for that
// reason are gone from what isoshelf serves.
func TestThePlainDiscShipsAndTheRemovedLogosDont(t *testing.T) {
	s := newServer(t, testDirs(t), "")
	rec := request(t, s, http.MethodGet, "/logo/disc.svg", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatalf("/logo/disc.svg: %d %.60s", rec.Code, rec.Body)
	}
	for _, slug := range []string{"windows", "windows11", "ubuntu", "atlasos", "nicehash"} {
		if _, err := staticFiles.ReadFile("static/logos/" + slug + ".svg"); err == nil {
			t.Errorf("%s.svg still ships inside isoshelf", slug)
		}
	}
}
