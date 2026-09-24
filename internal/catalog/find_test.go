package catalog

import (
	"strings"
	"testing"
)

// The where-to-find-it note (#58) loads, and one that isn't a line or two
// is refused: it is read beside a link, not as a page of instructions.
func TestTheFindNote(t *testing.T) {
	const note = `find = "The ISO is example-{version}-amd64.iso, in the releases folder."`
	anchor := `name = "Example Linux"`
	c, err := load(t, strings.Replace(validCatalog, anchor, anchor+"\n"+note, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Entry("example-x64").Find; !strings.Contains(got, "{version}") {
		t.Errorf("find is %q, want the note as written", got)
	}
	long := `find = "` + strings.Repeat("a", 401) + `"`
	if _, err := load(t, strings.Replace(validCatalog, anchor, anchor+"\n"+long, 1)); err == nil || !strings.Contains(err.Error(), "find:") {
		t.Errorf("a 401-character note loaded (%v), want it refused", err)
	}
}
