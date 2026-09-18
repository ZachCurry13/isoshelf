package catupdate

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

// published serves body as the project's catalog, and counts the requests.
func published(t *testing.T, body *string, requests *int) *remote.Client {
	t.Helper()
	client := remote.New("test")
	client.HTTP = &http.Client{Transport: remotetest.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests != nil {
			*requests++
		}
		if *body == "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(*body))
	}))}
	return client
}

// twoEntries is the smallest catalog that passes validation.
const twoEntries = `schema = 1

[[entry]]
id = "qubes"
name = "Qubes OS"
arch = "x86_64"
match = 'Qubes-R(?P<version>[\d.]+)-x86_64\.iso'
samples = ["Qubes-R4.2.0-x86_64.iso"]
page = "https://www.qubes-os.org/downloads/"
[entry.source]
type = "manual"

[[entry]]
id = "brand-new-image"
name = "Brand New Image"
arch = "x86_64"
match = 'brand-new-(?P<version>[\d.]+)\.iso'
samples = ["brand-new-1.0.iso"]
page = "https://example.org/"
[entry.source]
type = "manual"
`

func TestRefreshKeepsAGoodCatalog(t *testing.T) {
	dir := t.TempDir()
	body := twoEntries
	now := time.Now()

	result, err := Refresh(context.Background(), published(t, &body, nil), dir, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Entries != 2 {
		t.Fatalf("result = %+v, want 2 entries and a change", result)
	}
	if !hasString(result.Added, "Brand New Image") {
		t.Errorf("added = %v, want Brand New Image", result.Added)
	}
	if got := result.Summary(); !strings.Contains(got, "Brand New Image") {
		t.Errorf("summary = %q", got)
	}

	cat, err := Load(dir)
	if err != nil || cat == nil {
		t.Fatalf("loading what was saved: %v", err)
	}
	if cat.Entry("brand-new-image") == nil {
		t.Error("the new image isn't in the saved catalog")
	}

	// The same catalog again is not a change, and isn't downloaded twice a day.
	if Due(dir, now) {
		t.Error("another check is due straight away")
	}
	result, err = Refresh(context.Background(), published(t, &body, nil), dir, now.Add(25*time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Changed {
		t.Errorf("unchanged catalog reported as a change: %+v", result)
	}
}

func TestRefreshRefusesABrokenCatalog(t *testing.T) {
	dir := t.TempDir()
	good := twoEntries
	if _, err := Refresh(context.Background(), published(t, &good, nil), dir, time.Now(), true); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		body string
	}{
		{"not TOML at all", "<!doctype html><title>404</title>"},
		{"a misspelled key", strings.Replace(twoEntries, "arch =", "architecture =", 1)},
		{"a pattern that doesn't compile", strings.Replace(twoEntries, `match = 'Qubes-R(?P<version>[\d.]+)-x86_64\.iso'`, `match = 'Qubes-R(?P<version>[\d.+-x86_64\.iso'`, 1)},
		{"a sample that matches nothing", strings.Replace(twoEntries, `samples = ["Qubes-R4.2.0-x86_64.iso"]`, `samples = ["something-else.iso"]`, 1)},
		{"a schema from the future", strings.Replace(twoEntries, "schema = 1", "schema = 99", 1)},
		{"nothing at all", ""},
	} {
		body := tt.body
		_, err := Refresh(context.Background(), published(t, &body, nil), dir, time.Now(), true)
		if err == nil {
			t.Errorf("%s: accepted", tt.name)
		}
		// Whatever happened, the catalog already there still loads.
		cat, err := Load(dir)
		if err != nil || cat == nil || cat.Entry("qubes") == nil {
			t.Fatalf("%s: the good catalog was lost: %v", tt.name, err)
		}
	}
}

func TestRefreshWaitsADayBetweenChecks(t *testing.T) {
	dir := t.TempDir()
	body := twoEntries
	requests := 0
	client := published(t, &body, &requests)
	start := time.Now()

	if _, err := Refresh(context.Background(), client, dir, start, false); err != nil {
		t.Fatal(err)
	}
	for _, after := range []time.Duration{time.Minute, time.Hour, 23 * time.Hour} {
		result, err := Refresh(context.Background(), client, dir, start.Add(after), false)
		if err != nil || result != nil {
			t.Errorf("after %s: asked again (%+v, %v)", after, result, err)
		}
	}
	if requests != 1 {
		t.Errorf("%d requests in a day, want 1", requests)
	}

	// The button doesn't wait. It gets a fresh client, as the server gives it,
	// because a client remembers the documents it has already fetched.
	if _, err := Refresh(context.Background(), published(t, &body, &requests), dir, start.Add(time.Minute), true); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("%d requests, want the forced one too", requests)
	}
}

// The catalog isoshelf ships must be the one it publishes, or the first
// update would be a downgrade.
func TestPublishedCatalogIsTheBuiltInOne(t *testing.T) {
	built, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	shipped, err := os.ReadFile(filepath.Join("..", "catalog", "default.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(SourceURL, "/internal/catalog/default.toml") {
		t.Errorf("SourceURL %q no longer points at the catalog isoshelf ships", SourceURL)
	}

	dir := t.TempDir()
	body := string(shipped)
	if _, err := Refresh(context.Background(), published(t, &body, nil), dir, time.Now(), true); err != nil {
		t.Fatalf("the catalog in the repository wouldn't be accepted as an update: %v", err)
	}
	fetched, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched.Entries) != len(built.Entries) {
		t.Errorf("published catalog has %d entries, built-in has %d", len(fetched.Entries), len(built.Entries))
	}
}

func hasString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
