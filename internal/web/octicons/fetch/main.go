// Command fetch downloads the Octicons the page uses and writes
// static/octicons.svg and the licence beside it. Run from the repository root:
//
//	go run ./internal/web/octicons/fetch
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/web/octicons"
)

func get(c *http.Client, url string) (string, error) {
	resp, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return string(b), err
}

func main() {
	if _, err := os.Stat("internal/web/static"); err != nil {
		fmt.Fprintln(os.Stderr, "run this from the repository root:", err)
		os.Exit(1)
	}
	c := &http.Client{Timeout: 30 * time.Second}
	var symbols []string
	failed := 0
	for _, name := range octicons.Names {
		svg, err := get(c, octicons.SourceURL(name))
		if err == nil {
			var sym string
			if sym, err = octicons.Symbol(name, svg); err == nil {
				symbols = append(symbols, sym)
				continue
			}
		}
		fmt.Fprintf(os.Stderr, "FAIL  %-26s %v\n", name, err)
		failed++
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d icons failed; nothing written\n", failed)
		os.Exit(1)
	}
	licence, err := get(c, octicons.LicenseURL())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile("internal/web/static/octicons.svg", []byte(octicons.Sprite(symbols)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile("internal/web/octicons/LICENSE", []byte(licence), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d icons from Octicons %s written to internal/web/static/octicons.svg\n", len(symbols), octicons.Version)
}
