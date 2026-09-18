package web

import (
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/web/logos"
)

// logo serves a distro logo: one of the built-in ones, one fetched earlier,
// or one fetched now. Images without a logo get a 404 and the page draws
// coloured initials instead.
func (s *Server) logo(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSuffix(r.PathValue("slug"), ".svg")
	builtin, _ := fs.Sub(staticFiles, "static/logos")
	client := remote.New(s.cfg.Version)
	client.HTTP = s.cfg.HTTP
	store := &logos.Store{Builtin: builtin, Client: client}
	if s.cfg.Dirs.Config != "" {
		store.CacheDir = filepath.Join(s.cfg.Dirs.Config, "logos")
	}

	data, err := store.Get(r.Context(), slug)
	if errors.Is(err, logos.ErrNoLogo) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "couldn't fetch that logo", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(data)
}
