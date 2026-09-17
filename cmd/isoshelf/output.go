package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/check"
)

// writeTable prints a report for people.
func writeTable(w io.Writer, r *check.Report) {
	fmt.Fprintf(w, "%s (%s)\n\n", r.Target, r.Profile)
	if len(r.Items) == 0 {
		fmt.Fprintln(w, "No images found.")
	} else {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if r.Checked {
			fmt.Fprintln(tw, "STATUS\tTRACK\tVERSION\tLATEST\tFILE\tNOTE")
		} else {
			fmt.Fprintln(tw, "STATUS\tTRACK\tVERSION\tFILE\tNOTE")
		}
		for _, it := range r.Items {
			status := string(it.Status)
			if it.EOL && it.Status != check.EOL {
				status += " (EOL)"
			}
			track := "-"
			if it.Entry != nil {
				track = it.Entry.Name
			}
			if r.Checked {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", status, track, dash(it.Version), dash(it.Latest), dash(it.Path), it.Note)
			} else {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", status, track, dash(it.Version), dash(it.Path), it.Note)
			}
		}
		tw.Flush()
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, summary(r))
	for _, t := range r.Trash {
		fmt.Fprintf(w, "Trash folder %s uses %s. Emptying it frees that space.\n", t.Path, size(t.Bytes))
	}
	if len(r.Problems) > 0 {
		fmt.Fprintf(w, "Couldn't read %d path(s):\n", len(r.Problems))
		for _, p := range r.Problems {
			fmt.Fprintf(w, "  %s\n", p.Error())
		}
	}
}

// summary is a one-line count of the statuses, like
// "25 images: 9 update available, 1 EOL, 12 manual".
func summary(r *check.Report) string {
	counts := r.Counts()
	images := len(r.Items) - counts[check.Missing]
	var parts []string
	for _, s := range check.Order() {
		if n := counts[s]; n > 0 && s != check.Missing {
			label := string(s)
			if plural, ok := plurals[s]; ok && n != 1 {
				label = plural
			}
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	line := fmt.Sprintf("%d image(s)", images)
	if len(parts) > 0 {
		line += ": " + strings.Join(parts, ", ")
	}
	if n := counts[check.Missing]; n > 0 {
		line += fmt.Sprintf(". %d usually kept here but missing", n)
	}
	return line + "."
}

var plurals = map[check.Status]string{
	check.UpdateAvailable:  "updates available",
	check.ChecksumMismatch: "checksum mismatches",
	check.CheckFailed:      "checks failed",
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func size(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// The JSON output. Field names are part of isoshelf's interface for scripts,
// so change them with care.
type jsonReport struct {
	Target    string        `json:"target"`
	Profile   string        `json:"profile"`
	Checked   bool          `json:"checked"`
	Items     []jsonItem    `json:"items"`
	Trash     []jsonTrash   `json:"trash"`
	Problems  []jsonProblem `json:"problems"`
	AppUpdate *jsonUpdate   `json:"app_update,omitempty"`
}

type jsonItem struct {
	Path       string `json:"path,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Entry      string `json:"entry,omitempty"`
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	Status     string `json:"status"`
	EOL        bool   `json:"eol,omitempty"`
	Latest     string `json:"latest,omitempty"`
	LatestFile string `json:"latest_file,omitempty"`
	Note       string `json:"note,omitempty"`
}

type jsonTrash struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

type jsonProblem struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type jsonUpdate struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	URL     string `json:"url"`
}

func writeJSON(w io.Writer, r *check.Report, notice *appupdate.Notice) error {
	out := jsonReport{
		Target:   r.Target,
		Profile:  string(r.Profile),
		Checked:  r.Checked,
		Items:    []jsonItem{},
		Trash:    []jsonTrash{},
		Problems: []jsonProblem{},
	}
	for _, it := range r.Items {
		j := jsonItem{
			Path: it.Path, Size: it.Size, Kind: string(it.Kind), Name: it.Name(),
			Version: it.Version, Status: string(it.Status), EOL: it.EOL,
			Latest: it.Latest, LatestFile: it.LatestFile, Note: it.Note,
		}
		if it.Entry != nil {
			j.Entry = it.Entry.ID
		}
		out.Items = append(out.Items, j)
	}
	for _, t := range r.Trash {
		out.Trash = append(out.Trash, jsonTrash{Path: t.Path, Bytes: t.Bytes})
	}
	for _, p := range r.Problems {
		out.Problems = append(out.Problems, jsonProblem{Path: p.Path, Error: p.Err.Error()})
	}
	if notice != nil {
		out.AppUpdate = &jsonUpdate{Current: notice.Current, Latest: notice.Latest, URL: notice.URL}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
