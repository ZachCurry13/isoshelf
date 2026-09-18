package web

import (
	"net/http"
	"slices"
	"strings"
)

type catalogEntryJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Arch string `json:"arch"`
	// Size is roughly how big the download is, from the catalog.
	Size      int64  `json:"size,omitempty"`
	Popular   bool   `json:"popular,omitempty"`
	Updates   string `json:"updates"`
	Page      string `json:"page,omitempty"`
	Site      string `json:"site,omitempty"`
	Forum     string `json:"forum,omitempty"`
	Category  string `json:"category,omitempty"`
	Family    string `json:"family,omitempty"`
	Icon      string `json:"icon,omitempty"`
	IconColor string `json:"icon_color,omitempty"`
	OnTarget  bool   `json:"on_target"`
}

func (s *Server) getCatalog(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	onTarget := map[string]bool{}
	if s.report != nil {
		for _, it := range s.report.Items {
			if it.Entry != nil && it.Path != "" {
				onTarget[it.Entry.ID] = true
			}
		}
	}
	s.mu.Unlock()

	entries := []catalogEntryJSON{}
	cat := s.catalog()
	for i := range cat.Entries {
		e := &cat.Entries[i]
		entries = append(entries, catalogEntryJSON{
			ID: e.ID, Name: e.Name, Arch: e.Arch, Size: e.Size, Popular: e.Popular,
			Updates: e.Updates(), Page: e.Page,
			Site: e.Site, Forum: e.Forum, Category: e.Category, Family: e.Family,
			Icon: e.Icon, IconColor: e.IconColor, OnTarget: onTarget[e.ID],
		})
	}
	slices.SortFunc(entries, func(a, b catalogEntryJSON) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
