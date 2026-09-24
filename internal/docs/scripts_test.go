package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The page is one script per part, and some documents list them: CLAUDE.md,
// docs/design.md and docs/TODO.md do today, and CONTRIBUTING.md once did. A new script is easy to add and easy to
// forget in all three - report.js was missing from every one of them for four
// releases, and the lists read as complete, so nobody looking at them could
// tell. A document that names one script must name them all.
func TestEveryDocumentListingPageScriptsListsThemAll(t *testing.T) {
	root := repoRoot(t)
	names, err := filepath.Glob(filepath.Join(root, "internal", "web", "static", "*.js"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 {
		t.Fatalf("found %d page scripts; this test has lost track of where they are", len(names))
	}
	var scripts []string
	for _, name := range names {
		scripts = append(scripts, filepath.Base(name))
	}

	for _, doc := range []string{"CLAUDE.md", "docs/design.md", "docs/TODO.md", "README.md", "CONTRIBUTING.md", "SECURITY.md"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		// A document that doesn't get into the page's scripts at all is
		// nothing to do with this.
		if !strings.Contains(text, "app.js") {
			continue
		}
		var missing []string
		for _, script := range scripts {
			if !strings.Contains(text, script) {
				missing = append(missing, script)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s lists the page's scripts but leaves out %s. A list that reads as "+
				"complete and isn't is worse than no list: add them, or stop listing them.",
				doc, strings.Join(missing, ", "))
		}
	}
}
