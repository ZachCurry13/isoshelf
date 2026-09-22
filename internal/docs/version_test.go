package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The newest heading in CHANGELOG.md, like "## [v0.4.1] - 2026-09-22". The
// changelog is where a version is decided, so everything else follows it.
var changelogHeading = regexp.MustCompile(`(?m)^## \[(v\d+\.\d+\.\d+)\]`)

// A TrueNAS store entry names the exact image it packages, so two fields in
// deploy/truenas have to carry the version being released. They are the only
// files outside the changelogs that do, and the kind of thing nobody
// remembers at release time - so the build remembers instead.
func TestTheTrueNASEntryNamesThisVersion(t *testing.T) {
	root := repoRoot(t)

	body, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := changelogHeading.FindSubmatch(body)
	if m == nil {
		t.Fatal("CHANGELOG.md has no version heading of the form \"## [vX.Y.Z] - DATE\"")
	}
	version := string(m[1]) // v0.4.1

	for _, c := range []struct{ file, want string }{
		{"deploy/truenas/ix_values.yaml", "tag: " + version},
		{"deploy/truenas/app.yaml", "app_version: " + strings.TrimPrefix(version, "v")},
	} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), c.want) {
			t.Errorf("%s doesn't say %q. The newest CHANGELOG.md section is %s, and a "+
				"store entry that names an older image installs an older isoshelf.",
				c.file, c.want, version)
		}
	}
}
