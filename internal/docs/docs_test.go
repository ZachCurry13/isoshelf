// Package docs has no code. It holds the checks that keep what the
// repository says about itself true, because people read this project on
// GitHub without ever cloning it.
package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// assetName is a release file with a version in it, like
// isoshelf-v0.3.3-windows-amd64.exe. Every release renames these, so any
// document naming one is wrong the day after the next release.
var assetName = regexp.MustCompile(`isoshelf-v\d+\.\d+\.\d+`)

// keepsOldVersions are the files whose whole job is to say what each release
// held. Old version numbers are correct there and must not be updated.
var keepsOldVersions = map[string]bool{
	"CHANGELOG.md":         true,
	"CATALOG-CHANGES.md":   true,
	"docs/design-audit.md": true,
	"docs/archive.md":      true,
}

func TestNoDocumentNamesAVersionedReleaseFile(t *testing.T) {
	root := repoRoot(t)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md", ".yml", ".yaml":
		default:
			return nil
		}
		if keepsOldVersions[rel] {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(body), "\n") {
			if found := assetName.FindString(line); found != "" {
				t.Errorf("%s names the release file %q. Every release renames these, "+
					"so say which file by the part that doesn't change "+
					"(\"the file ending in -windows-amd64.exe\") instead:\n  %s",
					rel, found, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("can't find the repository root from here: %v", err)
	}
	return dir
}
