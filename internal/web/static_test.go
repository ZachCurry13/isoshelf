package web

import (
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// read returns one of the embedded static files.
func readStatic(t *testing.T, name string) string {
	t.Helper()
	data, err := fs.ReadFile(staticFiles, "static/"+name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// There is no JavaScript engine in these tests, so the page script can't be
// run. These checks catch the mistakes that would otherwise only show up as a
// blank page in front of someone: a script that lost its first line to a bad
// edit, or code reaching for an element that isn't there.
func TestPageScriptIsWellFormed(t *testing.T) {
	js := readStatic(t, "app.js")
	if !strings.HasPrefix(js, `"use strict";`) {
		first, _, _ := strings.Cut(js, "\n")
		t.Errorf("app.js should start with \"use strict\"; it starts with %q", first)
	}
	if n := strings.Count(js, "{") - strings.Count(js, "}"); n != 0 {
		t.Errorf("app.js has %d unclosed braces", n)
	}
	if n := strings.Count(js, "(") - strings.Count(js, ")"); n != 0 {
		t.Errorf("app.js has %d unclosed brackets", n)
	}
}

var (
	idPattern  = regexp.MustCompile(`\bid="([^"]+)"`)
	getPattern = regexp.MustCompile(`\$\("([^"]+)"\)`)
)

func TestScriptOnlyReachesForElementsThatExist(t *testing.T) {
	html := readStatic(t, "index.html")
	js := readStatic(t, "app.js")

	var ids []string
	for _, m := range idPattern.FindAllStringSubmatch(html, -1) {
		ids = append(ids, m[1])
	}
	if len(ids) < 20 {
		t.Fatalf("only %d ids found in index.html; the check is looking in the wrong place", len(ids))
	}

	seen := map[string]bool{}
	for _, m := range getPattern.FindAllStringSubmatch(js, -1) {
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		if !slices.Contains(ids, id) {
			t.Errorf("app.js asks for #%s, which index.html doesn't have", id)
		}
	}
}

// Every link opens in a new tab, and every new tab is opened safely: a page
// opened with target="_blank" can reach back through window.opener without
// rel="noopener".
func TestLinksOpenSafely(t *testing.T) {
	for _, name := range []string{"app.js", "index.html"} {
		body := readStatic(t, name)
		blanks := strings.Count(body, `target: "_blank"`) + strings.Count(body, `target="_blank"`)
		safe := strings.Count(body, `rel: "noopener noreferrer"`) + strings.Count(body, `rel="noopener noreferrer"`)
		if blanks != safe {
			t.Errorf("%s: %d links open in a new tab but %d say noopener noreferrer", name, blanks, safe)
		}
	}
}
