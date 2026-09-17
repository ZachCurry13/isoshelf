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

// jsonOutput is the --json output: the report plus the app update notice.
type jsonOutput struct {
	check.ReportJSON
	AppUpdate *appupdate.Notice `json:"app_update,omitempty"`
}

func writeJSON(w io.Writer, r *check.Report, notice *appupdate.Notice) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonOutput{ReportJSON: r.JSON(), AppUpdate: notice})
}
