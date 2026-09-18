package check

import (
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
)

// ReportJSON is a report as the CLI's --json output and the web UI show it.
// The field names are an interface for scripts, so change them with care.
type ReportJSON struct {
	Target   string        `json:"target"`
	Profile  string        `json:"profile"`
	Checked  bool          `json:"checked"`
	Items    []ItemJSON    `json:"items"`
	Trash    []TrashJSON   `json:"trash"`
	Problems []ProblemJSON `json:"problems"`
}

// ItemJSON is one row of a ReportJSON.
type ItemJSON struct {
	Path    string `json:"path,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Entry   string `json:"entry,omitempty"`
	Name    string `json:"name"`
	Arch    string `json:"arch,omitempty"`
	Page    string `json:"page,omitempty"`
	Updates string `json:"updates,omitempty"` // "download", "check-only" or "manual"
	Version string `json:"version,omitempty"`
	Status  string `json:"status"`
	EOL     bool   `json:"eol,omitempty"`
	Latest  string `json:"latest,omitempty"`
	// LatestFile is the newest file's name, when it can be downloaded.
	LatestFile string `json:"latest_file,omitempty"`
	Note       string `json:"note,omitempty"`
	// Modified is when the file last changed.
	Modified time.Time `json:"modified,omitzero"`
	// Category and Family group the image; the rest are for showing it.
	Category  string `json:"category,omitempty"`
	Family    string `json:"family,omitempty"`
	Icon      string `json:"icon,omitempty"`
	IconColor string `json:"icon_color,omitempty"`
	Site      string `json:"site,omitempty"`
	Forum     string `json:"forum,omitempty"`
	// Release is a page about the newest release.
	Release string `json:"release,omitempty"`
}

// TrashJSON is a trash folder and the space it uses.
type TrashJSON struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// ProblemJSON is a path that couldn't be read.
type ProblemJSON struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// JSON converts the report. Slices are never nil, so they encode as [].
func (r *Report) JSON() ReportJSON {
	out := ReportJSON{
		Target:   r.Target,
		Profile:  string(r.Profile),
		Checked:  r.Checked,
		Items:    []ItemJSON{},
		Trash:    []TrashJSON{},
		Problems: []ProblemJSON{},
	}
	for _, it := range r.Items {
		j := ItemJSON{
			Path: it.Path, Size: it.Size, Kind: string(it.Kind), Name: it.Name(),
			Version: it.Version, Status: string(it.Status), EOL: it.EOL,
			Latest: it.Latest, LatestFile: it.LatestFile, Note: it.Note,
			Modified: it.ModTime, Release: it.Release,
		}
		if e := it.Entry; e != nil {
			j.Entry, j.Arch, j.Page, j.Updates = e.ID, e.Arch, e.Page, e.Updates()
			j.Category, j.Family = e.Category, e.Family
			j.Icon, j.IconColor, j.Site, j.Forum = e.Icon, e.IconColor, e.Site, e.Forum
			if j.Updates != catalog.UpdatesDownload {
				j.LatestFile = ""
			}
		}
		out.Items = append(out.Items, j)
	}
	for _, t := range r.Trash {
		out.Trash = append(out.Trash, TrashJSON{Path: t.Path, Bytes: t.Bytes})
	}
	for _, p := range r.Problems {
		out.Problems = append(out.Problems, ProblemJSON{Path: p.Path, Error: p.Err.Error()})
	}
	return out
}
