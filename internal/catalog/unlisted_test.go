package catalog

import (
	"strings"
	"testing"
)

// An unlisted image (TODO item 8) still loads and still matches its file;
// only Add images leaves it out.
func TestAnUnlistedImageIsStillRecognized(t *testing.T) {
	anchor := `name = "Example Linux"`
	c, err := load(t, strings.Replace(validCatalog, anchor, anchor+"\nunlisted = true", 1))
	if err != nil {
		t.Fatal(err)
	}
	if e := c.Entry("example-x64"); e == nil || !e.Unlisted {
		t.Fatalf("entry is %+v, want it unlisted", e)
	}
	if m := c.Match("example-1.2-amd64.iso"); len(m) != 1 || m[0].Entry.ID != "example-x64" {
		t.Errorf("the file matched %v, want the unlisted entry", m)
	}
}
