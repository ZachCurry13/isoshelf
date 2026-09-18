// Command fetch downloads the distro logos that ship inside isoshelf, one per
// icon named in the built-in catalog:
//
//	go run ./internal/web/logos/fetch
//
// The logos come from Simple Icons (CC0 1.0). Entries without one, and
// catalogs the user adds later, fall back to the running app fetching a logo
// on demand or drawing coloured initials.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/web/logos"
)

func main() {
	out := filepath.Join("internal", "web", "static", "logos")
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "run this from the repository root:", err)
		os.Exit(1)
	}
	cat, err := catalog.Default()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var slugs, missing []string
	for i := range cat.Entries {
		if icon := cat.Entries[i].Icon; icon != "" && !slices.Contains(slugs, icon) {
			slugs = append(slugs, icon)
		} else if icon == "" {
			missing = append(missing, cat.Entries[i].ID)
		}
	}
	slices.Sort(slugs)

	client := &http.Client{Timeout: 30 * time.Second}
	failed := 0
	for _, slug := range slugs {
		body, err := download(client, logos.SourceURL(slug))
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %-16s %v\n", slug, err)
			failed++
			continue
		}
		if err := os.WriteFile(filepath.Join(out, slug+".svg"), body, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("ok    %-16s %d bytes\n", slug, len(body))
	}
	fmt.Printf("\n%d logos, %d entries without one (they get initials instead)\n", len(slugs), len(missing))
	if failed > 0 {
		os.Exit(1)
	}
}

func download(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, logos.MaxSize))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(body), "<svg") {
		return nil, fmt.Errorf("not an SVG")
	}
	return body, nil
}
