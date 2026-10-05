package catalog

import (
	"strings"
	"testing"
	"testing/fstest"
)

// A checksum name may use a named group from artifact.file (v0.8.9, #6),
// and only one that is there: a misspelt group is still refused.
func TestAChecksumNameMayUseAGroupFromTheFile(t *testing.T) {
	entry := func(manifest string) string {
		return `schema = 1
[[entry]]
id = "example"
name = "Example"
arch = "x86_64"
match = 'Example-Live-(?P<version>\d+-[\d.]+)\.iso'
samples = ["Example-Live-44-1.7.iso"]
[entry.source]
type = "listing"
url = "https://example.org/"
regex = '(?P<version>\d+)'
[entry.artifact]
base = "https://example.org/releases/{version}/"
file = 'Example-Live-{version}-(?P<compose>[\d.]+)\.iso'
manifest = "` + manifest + `"
`
	}
	load := func(text string) error {
		_, err := Load(fstest.MapFS{"c.toml": {Data: []byte(text)}}, "c.toml")
		return err
	}
	if err := load(entry("Example-{version}-{compose}-CHECKSUM")); err != nil {
		t.Errorf("a group from the file was refused: %v", err)
	}
	if err := load(entry("Example-{version}-{compos}-CHECKSUM")); err == nil || !strings.Contains(err.Error(), "{compos} cannot be used here") {
		t.Errorf("a misspelt group loaded (%v), want it refused", err)
	}
}
