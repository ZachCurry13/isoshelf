package web

import (
	"net/http"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
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
	// AnyHost lets the page be opened by the machine's name or address on the
	// network rather than only by localhost. It is set when isoshelf was told
	// to listen somewhere other than loopback - in a container, mostly - and
	// it is the only thing that changes about who may connect. The token, the
	// cookie, the header on every change and the same-origin check all still
	// apply, and they are what actually keeps other people out.
	AnyHost bool
	// Now defaults to time.Now.
	Now func() time.Time
	// SelfUpdate is what isoshelf needs to update its own program.
	SelfUpdate SelfUpdateConfig
}
