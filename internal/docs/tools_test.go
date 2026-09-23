package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The tools that go with the images are written twice: in README.md, where
// somebody deciding whether to use isoshelf reads, and on the page, where the
// people who already have it are. Two copies of anything is normally exactly
// what this repository refuses to do - so the two are checked against each
// other here, which is what makes the second copy affordable.
//
// Only the names and the addresses are compared. The sentence beside each one
// is written for where it sits, and saying the same thing twice in the same
// words is not the point.
func TestTheToolsListSaysTheSameThingInBothPlaces(t *testing.T) {
	root := repoRoot(t)

	page := section(t, filepath.Join(root, "internal", "web", "static", "index.html"),
		`<ul class="tools">`, `</ul>`)
	readme := section(t, filepath.Join(root, "README.md"),
		"### Tools that go with these", "\nThe same tools are listed on the page")

	onPage := links(regexp.MustCompile(`<a href="([^"]+)"[^>]*>([^<]+)</a>`), page, 2, 1)
	inReadme := links(regexp.MustCompile(`\*\*\[([^\]]+)\]\(([^)]+)\)\*\*`), readme, 1, 2)

	if len(onPage) == 0 {
		t.Fatal("no tools found on the page; this test has lost track of how they are written")
	}
	if len(onPage) != len(inReadme) {
		t.Fatalf("the page lists %d tools and README.md lists %d:\n  page:   %v\n  readme: %v",
			len(onPage), len(inReadme), onPage, inReadme)
	}
	for i := range onPage {
		if onPage[i] != inReadme[i] {
			t.Errorf("tool %d differs:\n  page:   %s\n  readme: %s\n"+
				"Both lists name the same tools, in the same order, at the same addresses.",
				i+1, onPage[i], inReadme[i])
		}
	}

	// No version and no download: the moment isoshelf tracks either, it owns
	// that tool's release notes forever.
	for _, where := range []struct{ name, text string }{{"the page", page}, {"README.md", readme}} {
		if m := regexp.MustCompile(`\b\d+\.\d+(\.\d+)?\b`).FindString(where.text); m != "" {
			t.Errorf("%s puts a version (%s) in the tools list; it will go stale", where.name, m)
		}
		if m := regexp.MustCompile(`(?i)\b(download|\.exe|\.dmg|\.appimage)\b`).FindString(where.text); m != "" {
			t.Errorf("%s offers a download (%s) in the tools list; link to the project instead",
				where.name, m)
		}
	}
}

// section returns the text between two markers, failing when either is gone.
func section(t *testing.T, path, from, to string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	i := indexOrFail(t, path, text, from)
	rest := text[i+len(from):]
	return rest[:indexOrFail(t, path, rest, to)]
}

func indexOrFail(t *testing.T, path, text, want string) int {
	t.Helper()
	for i := 0; i+len(want) <= len(text); i++ {
		if text[i:i+len(want)] == want {
			return i
		}
	}
	t.Fatalf("%s no longer contains %q; if the tools list moved, move this test with it", path, want)
	return 0
}

// links pulls "name <url>" pairs out with one regexp, sorted so the two lists
// can be compared whatever order each was written in.
func links(re *regexp.Regexp, text string, nameGroup, urlGroup int) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		// A non-breaking space keeps a tool's name from wrapping on the page;
		// it is the same name either way.
		name := strings.ReplaceAll(m[nameGroup], "\u00a0", " ")
		out = append(out, name+" <"+m[urlGroup]+">")
	}
	sort.Strings(out)
	return out
}
