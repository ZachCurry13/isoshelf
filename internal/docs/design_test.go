package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/web/primer"
)

// docs/design-system.md is how somebody who is changing the look finds out
// what there is to change. It lists every token and every stylesheet, and a
// list that is missing the newest one reads as complete and sends them the
// wrong way, so these fail when the document and the files drift apart.

func designDoc(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "design-system.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestDesignSystemNamesEveryToken(t *testing.T) {
	doc := designDoc(t)
	css, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "web", "static", "css", "tokens.css"))
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for name := range primer.Defined(string(css)) {
		if !strings.Contains(doc, "`"+name+"`") {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("docs/design-system.md doesn't describe %s (css/tokens.css defines them). "+
			"Add each to the token table, with what it is for.", strings.Join(missing, ", "))
	}
}

func TestDesignSystemNamesEveryStylesheet(t *testing.T) {
	doc := designDoc(t)
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "internal", "web", "static", "css", "*.css"))
	if err != nil || len(files) < 5 {
		t.Fatalf("found %d stylesheets (%v); this test has lost track of where they are", len(files), err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		if !strings.Contains(doc, "`"+name+"`") {
			t.Errorf("docs/design-system.md doesn't say what css/%s styles", name)
		}
	}
}

// The document names the Primer release the tokens come from, so bumping it
// without reading the document is caught.
func TestDesignSystemNamesThePinnedPrimerVersion(t *testing.T) {
	if !strings.Contains(designDoc(t), "@primer/primitives "+primer.Version) {
		t.Errorf("docs/design-system.md should say it uses @primer/primitives %s; update it with the version", primer.Version)
	}
}

// Every test the document says enforces a rule has to exist, in the web
// package's tests.
func TestDesignSystemOnlyNamesTestsThatExist(t *testing.T) {
	doc := designDoc(t)
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "internal", "web", "*_test.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no test files found in internal/web: %v", err)
	}
	var web strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		web.Write(b)
	}
	for _, m := range regexp.MustCompile("`(Test[A-Za-z]+)`").FindAllStringSubmatch(doc, -1) {
		if !strings.Contains(web.String(), "func "+m[1]+"(") {
			t.Errorf("docs/design-system.md names %s, which isn't in a test in internal/web", m[1])
		}
	}
}
