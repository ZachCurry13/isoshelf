// Command record checks every entry of the built-in catalog against the live
// sites, the same way "isoshelf check" does, reports the ones that fail, and
// saves each response it reads for the tests to replay:
//
//	go run ./internal/remote/remotetest/record
//
// It is the only part of isoshelf's tests that goes online, and it only runs
// when started by hand. It replaces the whole recorded folder, so responses
// no entry needs any more disappear; if a run goes wrong, restore the folder
// with git. Responses are trimmed to what isoshelf reads, to keep the
// repository small.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/source"
)

var sizes = flag.Bool("sizes", false, "also measure how big each image is, for the catalog's size hints")

func main() {
	flag.Parse()
	measured := map[string]int64{}
	out := filepath.Join("internal", "remote", "remotetest", "recorded")
	if _, err := os.Stat(out); err != nil {
		fmt.Fprintln(os.Stderr, "run this from the repository root:", err)
		os.Exit(1)
	}
	cat, err := catalog.Default()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.RemoveAll(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client := remote.New("recorder")
	client.HTTP = &http.Client{Timeout: time.Minute, Transport: &recorder{base: http.DefaultTransport, dir: out}}
	client.GitHubToken = os.Getenv("GITHUB_TOKEN")
	ctx := context.Background()

	failed := 0
	for i := range cat.Entries {
		e := &cat.Entries[i]
		if e.Source.Type == catalog.SourceManual {
			continue
		}
		rel, err := source.Latest(ctx, client, e)
		file := ""
		var art *resolve.Artifact
		if err == nil {
			art, err = resolve.Resolve(ctx, client, e, rel)
			switch {
			case errors.Is(err, resolve.ErrNoArtifact):
				err, file = nil, "(check-only)"
			case err == nil:
				file = art.Filename
			}
		}
		if err != nil {
			failed++
			fmt.Printf("FAIL  %-28s %v\n", e.ID, err)
			continue
		}
		fmt.Printf("ok    %-28s %-12s %s\n", e.ID, rel.Version, file)
		if *sizes && art != nil {
			if size := measure(ctx, art); size > 0 {
				measured[e.ID] = size
			}
		}
	}
	if len(measured) > 0 {
		fmt.Printf("\nSizes, for the size = lines in the catalog:\n")
		for _, id := range slices.Sorted(maps.Keys(measured)) {
			fmt.Printf("size  %-28s %d\n", id, measured[id])
		}
	}
	if failed > 0 {
		fmt.Printf("\n%d entries failed.\n", failed)
		os.Exit(1)
	}
}

// measure asks how big an image is without downloading it. The catalog keeps
// the answer as a hint, so the page can say "about 4.7 GB" and warn when a
// download wouldn't fit. GitHub already says, so nothing is asked of it.
func measure(ctx context.Context, art *resolve.Artifact) int64 {
	if art.Size > 0 {
		return art.Size
	}
	if len(art.URLs) == 0 {
		return 0
	}
	client := &http.Client{Timeout: time.Minute}
	url := art.URLs[0]

	if size := contentLength(ctx, client, http.MethodHead, url, ""); size > 0 {
		return size
	}
	// Some servers refuse HEAD. Asking for the first byte gets the length in
	// a Content-Range header instead, and downloads nothing worth mentioning.
	return contentLength(ctx, client, http.MethodGet, url, "bytes=0-0")
}

func contentLength(ctx context.Context, client *http.Client, method, url, rang string) int64 {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "isoshelf-recorder")
	if rang != "" {
		req.Header.Set("Range", rang)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer func() {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0
	}
	if resp.StatusCode == http.StatusPartialContent {
		// "bytes 0-0/4556128256"
		if _, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok {
			size, err := strconv.ParseInt(strings.TrimSpace(total), 10, 64)
			if err == nil {
				return size
			}
		}
		return 0
	}
	return resp.ContentLength
}

// recorder is a transport that saves every successful response it passes on.
type recorder struct {
	base http.RoundTripper
	dir  string
	mu   sync.Mutex
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if loc := resp.Header.Get("Location"); resp.StatusCode >= 300 && resp.StatusCode < 400 && loc != "" {
		// Tests can't replay redirects, and checksum files must not redirect
		// to another host, so the catalog should use the final address.
		fmt.Printf("note  %s redirects (%d) to %s; use that address in the catalog\n", req.URL, resp.StatusCode, loc)
	}
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	saved, err := trim(req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("trimming %s: %w", req.URL, err)
	}
	name := filepath.Join(r.dir, filepath.FromSlash(remotetest.Name(req.URL)))
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return nil, err
	}
	return resp, os.WriteFile(name, saved, 0o644)
}

// trim keeps the parts of large responses that isoshelf reads.
func trim(u *url.URL, body []byte) ([]byte, error) {
	switch {
	case u.Host == "api.github.com" && strings.HasSuffix(u.Path, "/releases"):
		return releases(body, 10)
	case u.Host == "manjaro.org":
		return links(body, `https://download\.manjaro\.org/[^"]+`), nil
	case u.Host == "sourceforge.net" && strings.HasSuffix(u.Path, "/rss"):
		return rssItems(body, 6), nil
	}
	return body, nil
}

var imageAsset = regexp.MustCompile(`\.(iso|img|xz|gz|zip|7z)$`)

// releases keeps the first n GitHub releases, the fields isoshelf reads, and
// assets that look like images.
func releases(body []byte, n int) ([]byte, error) {
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
			if name, _ := asset["name"].(string); imageAsset.MatchString(name) {
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

// links keeps only the links whose href matches hrefPattern.
func links(body []byte, hrefPattern string) []byte {
	re := regexp.MustCompile(`<a[^>]*href="` + hrefPattern + `"[^>]*>`)
	var b bytes.Buffer
	b.WriteString("<html><body>\n")
	for _, m := range re.FindAll(body, -1) {
		b.Write(m)
		b.WriteString("\n")
	}
	b.WriteString("</body></html>\n")
	return b.Bytes()
}

// rssItems keeps the first n items of an RSS feed.
func rssItems(body []byte, n int) []byte {
	parts := strings.Split(string(body), "<item>")
	if len(parts) <= n+1 {
		return body
	}
	return []byte(strings.Join(parts[:n+1], "<item>") + "</channel></rss>\n")
}
