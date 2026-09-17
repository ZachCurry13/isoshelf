package catalog

import "testing"

// The built-in catalog ships inside the binary, so it must always be valid.
func TestDefaultCatalog(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		filename string
		wantID   string // empty: no entry matches
	}{
		{"netboot.xyz.iso", "netbootxyz"},
		{"netboot.xyz-multiarch.iso", ""}, // not in the catalog yet
		{"Windows.iso", ""},               // ambiguous, needs Assign
		{"notes.txt", ""},
	}
	for _, tt := range tests {
		matches := c.Match(tt.filename)
		switch {
		case tt.wantID == "" && len(matches) != 0:
			t.Errorf("%s: matched %q, want no match", tt.filename, matches[0].Entry.ID)
		case tt.wantID != "" && (len(matches) != 1 || matches[0].Entry.ID != tt.wantID):
			t.Errorf("%s: got %d matches, want exactly %q", tt.filename, len(matches), tt.wantID)
		}
	}
}
