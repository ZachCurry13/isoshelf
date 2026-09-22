package web

import (
	"net/http"
	"runtime"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
)

// Reporting a problem.
//
// isoshelf sends nothing anywhere. When something goes wrong the page offers
// to open GitHub's own bug form with the details already filled in: the
// person reads the whole thing in front of them, edits or deletes any of it,
// and presses submit themselves - or closes the tab, and nothing has left
// their computer. That is the whole mechanism. There is no server to run, no
// telemetry to explain, and nothing to opt out of.
//
// This endpoint only says what isoshelf knows about itself. What it does NOT
// include is deliberate: no folder paths, no file names, no drive letters,
// no user name. A path is the one thing in this app most likely to carry
// somebody's name, and a bug report is a public web page.

// NewIssueURL is where GitHub's bug form lives. The page adds the fields.
const NewIssueURL = "https://github.com/" + appupdate.Repo + "/issues/new"

// reportJSON is what the page shows before anything is sent, field by field.
type reportJSON struct {
	// Version is the isoshelf being run.
	Version string `json:"version"`
	// OS is "Windows", "Linux" or "Somewhere else", to match the dropdown in
	// the bug form.
	OS string `json:"os"`
	// System is the plain detail: operating system, processor and the Go it
	// was built with.
	System string `json:"system"`
	// Folder is the kind of folder, never which one: "Ventoy USB drive",
	// "Proxmox ISO storage", "Folder of images".
	Folder string `json:"folder"`
	// Template is the bug form to open.
	Template string `json:"template"`
	// URL is the bug form itself.
	URL string `json:"url"`
}

var profileWords = map[string]string{
	"ventoy":  "Ventoy USB drive",
	"proxmox": "Proxmox ISO storage",
	"folder":  "Folder of images",
}

var osWords = map[string]string{
	"windows": "Windows",
	"linux":   "Linux",
}

// getReport describes this isoshelf, for a bug report the user sends himself.
func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	profile := ""
	if s.st != nil {
		profile = string(s.st.Profile)
	}
	s.mu.Unlock()

	where, known := osWords[runtime.GOOS]
	if !known {
		where = "Somewhere else"
	}
	out := reportJSON{
		Version:  s.cfg.Version,
		OS:       where,
		System:   runtime.GOOS + " " + runtime.GOARCH + ", built with " + runtime.Version(),
		Folder:   profileWords[profile],
		Template: "bug.yml",
		URL:      NewIssueURL,
	}
	writeJSON(w, http.StatusOK, out)
}
