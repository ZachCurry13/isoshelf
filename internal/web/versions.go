package web

import (
	"errors"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/source"
)

// versionJSON is one version a project still publishes, for "Install an
// older version" in the details panel (#2).
type versionJSON struct {
	Version string `json:"version"`
	// Newest marks the one an ordinary update installs.
	Newest bool `json:"newest,omitempty"`
	// EOL marks a release cycle the project no longer supports, so going
	// back to it can say so.
	EOL bool `json:"eol,omitempty"`
}

// getVersions asks the image's project which versions it still publishes,
// newest first. It answers "why" instead when there is nothing to choose
// from: an image isoshelf only links to, or only checks, has no older
// version it could download either.
func (s *Server) getVersions(w http.ResponseWriter, r *http.Request) {
	entry := s.catalog().Entry(r.URL.Query().Get("entry"))
	if entry == nil {
		writeError(w, http.StatusBadRequest, "isoshelf doesn't know that image. Scan the folder again.")
		return
	}
	if entry.Updates() != catalog.UpdatesDownload {
		writeJSON(w, http.StatusOK, map[string]any{
			"versions": []versionJSON{},
			"why":      "isoshelf doesn't download " + entry.Name + ", so it can't offer an older version. The project's own page may list them.",
		})
		return
	}
	all, err := source.Versions(r.Context(), s.client(), entry)
	if errors.Is(err, source.ErrManual) {
		writeJSON(w, http.StatusOK, map[string]any{"versions": []versionJSON{}, "why": "The project's page is the only place to get " + entry.Name + "."})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't ask the project which versions it publishes ("+err.Error()+"). Check the connection and try again.")
		return
	}
	out := make([]versionJSON, 0, len(all))
	for i, rel := range all {
		v := versionJSON{Version: rel.Version, Newest: i == 0}
		for _, c := range rel.Cycles {
			if c.Name == rel.Cycle {
				v.EOL = c.EOL
			}
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": out})
}
