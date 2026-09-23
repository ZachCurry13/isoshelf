package docs

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// A number written in a document is a promise to keep it current, and the
// README's roadmap made that promise about the catalog and then broke it
// quietly: "86 so far" is the sort of sentence nobody re-reads. Counting is
// something a test can do, so it does.
//
// The lesson is wider than this one number, and is in CLAUDE.md: the
// roadmap's "next" and "later" lines went eleven releases naming work that
// had already shipped, and no test can tell that a plan has come true. This
// test covers the part that can be checked; re-reading the roadmap before a
// release covers the rest.
func TestTheREADMECountsTheCatalogCorrectly(t *testing.T) {
	root := repoRoot(t)

	catalog, err := os.ReadFile(filepath.Join(root, "internal", "catalog", "default.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, line := range bytes.Split(catalog, []byte("\n")) {
		if bytes.Equal(bytes.TrimSpace(line), []byte("[[entry]]")) {
			want++
		}
	}

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(\d+) so far`).FindSubmatch(readme)
	if m == nil {
		t.Fatal(`README.md no longer says "N so far" about the catalog; if that sentence went, delete this test with it`)
	}
	got, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("README.md says the catalog holds %d images; default.toml holds %d", got, want)
	}
}
