// Command record refreshes the recorded responses that tests replay. It is
// the only part of isoshelf's test setup that uses the live network, and it
// only runs when started by hand:
//
//	go run ./internal/remote/remotetest/record
//
// Responses are trimmed to what isoshelf reads, to keep the repository small.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

// recording is one URL to record and how to trim its response.
type recording struct {
	url  string
	trim func([]byte) ([]byte, error)
}

var recordings = []recording{
	{"https://endoflife.date/api/v1/products/linuxmint/", nil},
	{"https://endoflife.date/api/v1/products/mxlinux/", nil},
	{"https://endoflife.date/api/v1/products/centos/", nil},
	{"https://api.pop-os.org/builds/22.04/intel", nil},
	{"https://iso.pop-os.org/22.04/amd64/intel/58/SHA256SUMS", nil},
	{"https://mirror.cachyos.org/ISO/desktop/", nil},
	{"https://mirror.cachyos.org/ISO/desktop/260809/cachyos-desktop-linux-260809.iso.sha256", nil},
	{"https://download.bazzite.gg/bazzite-deck-gnome-stable-live-amd64.iso-CHECKSUM", nil},
	{"https://mirrors.kernel.org/linuxmint/stable/22.3/sha256sum.txt", nil},
	{"https://vault.centos.org/altarch/7.9.2009/isos/i386/sha256sum.txt", nil},
	{"https://api.github.com/repos/netbootxyz/netboot.xyz/releases?per_page=100", releases(3, `\.iso$|checksums`)},
	{"https://api.github.com/repos/ublue-os/bazzite/releases?per_page=100", releases(6, `^$`)},
	{"https://manjaro.org/products/download/x86", links(`https://download\.manjaro\.org/[^"]+`)},
	{"https://sourceforge.net/projects/clonezilla/rss?path=/clonezilla_live_alternative", rssItems(6)},
}

func main() {
	out := filepath.Join("internal", "remote", "remotetest", "recorded")
	if _, err := os.Stat(out); err != nil {
		fmt.Fprintln(os.Stderr, "run this from the repository root:", err)
		os.Exit(1)
	}
	failed := false
	for _, r := range recordings {
		if err := record(out, r); err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func record(out string, r recording) error {
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "isoshelf-test-recorder (+https://github.com/ZachCurry13/isoshelf)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", r.url, resp.Status)
	}
	if r.trim != nil {
		if body, err = r.trim(body); err != nil {
			return fmt.Errorf("%s: %w", r.url, err)
		}
	}
	u, _ := url.Parse(r.url)
	name := filepath.Join(out, filepath.FromSlash(remotetest.Name(u)))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	fmt.Printf("%7d  %s\n", len(body), remotetest.Name(u))
	return os.WriteFile(name, body, 0o644)
}

// releases keeps the first n GitHub releases, the fields isoshelf reads, and
// the assets whose names match assetPattern.
func releases(n int, assetPattern string) func([]byte) ([]byte, error) {
	re := regexp.MustCompile(assetPattern)
	return func(body []byte) ([]byte, error) {
		var all []map[string]any
		if err := json.Unmarshal(body, &all); err != nil {
			return nil, err
		}
		var kept []map[string]any
		for _, r := range all[:min(n, len(all))] {
			release := map[string]any{}
			for _, k := range []string{"tag_name", "name", "draft", "prerelease", "published_at", "html_url"} {
				release[k] = r[k]
			}
			assets := []map[string]any{}
			list, _ := r["assets"].([]any)
			for _, a := range list {
				asset, _ := a.(map[string]any)
				if name, _ := asset["name"].(string); re.MatchString(name) {
					assets = append(assets, map[string]any{
						"name": asset["name"], "size": asset["size"],
						"digest": asset["digest"], "browser_download_url": asset["browser_download_url"],
					})
				}
			}
			release["assets"] = assets
			kept = append(kept, release)
		}
		return json.MarshalIndent(kept, "", "  ")
	}
}

// links keeps only the links whose href matches hrefPattern.
func links(hrefPattern string) func([]byte) ([]byte, error) {
	re := regexp.MustCompile(`<a[^>]*href="` + hrefPattern + `"[^>]*>`)
	return func(body []byte) ([]byte, error) {
		var b bytes.Buffer
		b.WriteString("<html><body>\n")
		for _, m := range re.FindAll(body, -1) {
			b.Write(m)
			b.WriteString("\n")
		}
		b.WriteString("</body></html>\n")
		return b.Bytes(), nil
	}
}

// rssItems keeps the first n items of an RSS feed.
func rssItems(n int) func([]byte) ([]byte, error) {
	return func(body []byte) ([]byte, error) {
		parts := strings.Split(string(body), "<item>")
		if len(parts) <= n+1 {
			return body, nil
		}
		return []byte(strings.Join(parts[:n+1], "<item>") + "</channel></rss>\n"), nil
	}
}
