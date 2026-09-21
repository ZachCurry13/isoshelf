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

// scripts are the page's own JavaScript files, in the order index.html loads
// them. Everything below checks all of them: a mistake in the newer file
// leaves just as blank a page as one in the older.
var scripts = []string{
	"app.js", "images.js", "details.js", "downloads.js", "actions.js", "folders.js", "archive.js", "settings.js",
	"upload.js",
}

// There is no JavaScript engine in these tests, so the page scripts can't be
// run. These checks catch the mistakes that would otherwise only show up as a
// blank page in front of someone: a script that lost its first line to a bad
// edit, or code reaching for an element that isn't there.
func TestPageScriptsAreWellFormed(t *testing.T) {
	for _, name := range scripts {
		js := readStatic(t, name)
		if !strings.HasPrefix(js, `"use strict";`) {
			first, _, _ := strings.Cut(js, "\n")
			t.Errorf("%s should start with \"use strict\"; it starts with %q", name, first)
		}
		if n := strings.Count(js, "{") - strings.Count(js, "}"); n != 0 {
			t.Errorf("%s has %d unclosed braces", name, n)
		}
		if n := strings.Count(js, "(") - strings.Count(js, ")"); n != 0 {
			t.Errorf("%s has %d unclosed brackets", name, n)
		}
	}
}

// Every script the page loads has to be one isoshelf actually ships, and
// every script it ships has to be one the page loads.
func TestPageLoadsEveryScript(t *testing.T) {
	html := readStatic(t, "index.html")
	for _, name := range scripts {
		if !strings.Contains(html, `src="/static/`+name+`"`) {
			t.Errorf("index.html doesn't load %s", name)
		}
		if _, err := fs.ReadFile(staticFiles, "static/"+name); err != nil {
			t.Errorf("%s isn't there: %v", name, err)
		}
	}
	found, err := fs.Glob(staticFiles, "static/*.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range found {
		name := strings.TrimPrefix(path, "static/")
		if !slices.Contains(scripts, name) {
			t.Errorf("static/%s is shipped but this test doesn't know about it", name)
		}
	}
}

var (
	idPattern  = regexp.MustCompile(`\bid="([^"]+)"`)
	getPattern = regexp.MustCompile(`\$\("([^"]+)"\)`)
)

func TestScriptsOnlyReachForElementsThatExist(t *testing.T) {
	html := readStatic(t, "index.html")

	var ids []string
	for _, m := range idPattern.FindAllStringSubmatch(html, -1) {
		ids = append(ids, m[1])
	}
	if len(ids) < 20 {
		t.Fatalf("only %d ids found in index.html; the check is looking in the wrong place", len(ids))
	}

	seen := map[string]bool{}
	for _, name := range scripts {
		for _, m := range getPattern.FindAllStringSubmatch(readStatic(t, name), -1) {
			id := m[1]
			if seen[id] {
				continue
			}
			seen[id] = true
			if !slices.Contains(ids, id) {
				t.Errorf("%s asks for #%s, which index.html doesn't have", name, id)
			}
		}
	}
}

// Every link opens in a new tab, and every new tab is opened safely: a page
// opened with target="_blank" can reach back through window.opener without
// rel="noopener".
func TestLinksOpenSafely(t *testing.T) {
	for _, name := range append(slices.Clone(scripts), "index.html") {
		body := readStatic(t, name)
		blanks := strings.Count(body, `target: "_blank"`) + strings.Count(body, `target="_blank"`)
		safe := strings.Count(body, `rel: "noopener noreferrer"`) + strings.Count(body, `rel="noopener noreferrer"`)
		if blanks != safe {
			t.Errorf("%s: %d links open in a new tab but %d say noopener noreferrer", name, blanks, safe)
		}
	}
}
