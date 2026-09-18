package usercat

import (
	"os"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
)

func builtIn(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestAddNamesAFile(t *testing.T) {
	dir := t.TempDir()
	base := builtIn(t)

	mine, err := Add(dir, base, Image{
		Name:     "My Homebrew Build",
		Filename: "MyHomebrew_Build_x64.iso",
		Category: "other",
	})
	if err != nil {
		t.Fatal(err)
	}
	entry := mine.Entry("my-my-homebrew-build")
	if entry == nil {
		t.Fatalf("entry not found; file holds:\n%s", read(t, dir))
	}
	if entry.Name != "My Homebrew Build" || entry.Arch != "x86_64" || entry.Source.Type != catalog.SourceManual {
		t.Errorf("entry = %+v", entry)
	}

	// It matches that file and nothing else, not even a similar name.
	merged, err := catalog.Merge(base, mine)
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Match("MyHomebrew_Build_x64.iso"); len(got) != 1 || got[0].Entry.ID != entry.ID {
		t.Errorf("matching the named file gave %d entries", len(got))
	}
	if got := merged.Match("MyHomebrew_Build_x64_v2.iso"); len(got) != 0 {
		t.Errorf("a longer name matched too: %d entries", len(got))
	}

	// A second image goes in beside the first.
	mine, err = Add(dir, merged, Image{Name: "Old Install Disc", Filename: "disc1.iso", Arch: "x86"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mine.Entries) != 2 {
		t.Errorf("file holds %d entries, want 2:\n%s", len(mine.Entries), read(t, dir))
	}
}

func TestAddLeavesNothingBehindWhenItFails(t *testing.T) {
	dir := t.TempDir()
	base := builtIn(t)
	mine, err := Add(dir, base, Image{Name: "Keep Me", Filename: "keep-me.iso"})
	if err != nil {
		t.Fatal(err)
	}
	good := read(t, dir)
	// Later attempts see the entry already added, as the server does.
	base, err = catalog.Merge(base, mine)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		img  Image
		want string
	}{
		{"no name", Image{Filename: "x.iso"}, "give the image a name"},
		{"no file", Image{Name: "Something"}, "no file"},
		{"a path, not a name", Image{Name: "Something", Filename: "sub/x.iso"}, "not a path"},
		{"unknown architecture", Image{Name: "Something", Filename: "x.iso", Arch: "risc"}, "architecture"},
		{"unknown kind", Image{Name: "Something", Filename: "x.iso", Category: "games"}, "kind"},
		{"a link that isn't one", Image{Name: "Something", Filename: "x.iso", Page: "example.org"}, "https://"},
		{"the same name twice", Image{Name: "Keep Me", Filename: "other.iso"}, "already an image called"},
		// A name that would claim a file the published catalog already knows.
		{"a file the catalog knows", Image{Name: "Not Netboot", Filename: "netboot.xyz.iso"}, "matches"},
	}
	for _, tt := range tests {
		_, err := Add(dir, base, tt.img)
		if err == nil {
			t.Errorf("%s: accepted", tt.name)
		} else if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %v, want something about %q", tt.name, err, tt.want)
		}
		if got := read(t, dir); got != good {
			t.Fatalf("%s: the file was left changed:\n%s", tt.name, got)
		}
	}
}

// The first failure must not leave a file with only a header in it.
func TestAddFailingFirstLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := Add(dir, builtIn(t), Image{Name: "", Filename: "x.iso"}); err == nil {
		t.Fatal("accepted an image with no name")
	}
	if _, err := os.Stat(Path(dir)); err == nil {
		t.Errorf("%s was created anyway:\n%s", FileName, read(t, dir))
	}
	mine, err := Load(dir)
	if err != nil || mine != nil {
		t.Errorf("Load = %v, %v; want nothing", mine, err)
	}
}

func TestID(t *testing.T) {
	for name, want := range map[string]string{
		"Windows Server 2022": "my-windows-server-2022",
		"  Odd   Name!!  ":    "my-odd-name",
		"Pop!_OS":             "my-pop-os",
		"???":                 "my-image",
	} {
		if got := ID(name); got != want {
			t.Errorf("ID(%q) = %q, want %q", name, got, want)
		}
		if !Mine(ID(name)) {
			t.Errorf("ID(%q) is not recognized as the user's own", name)
		}
	}
	if Mine("ubuntu-desktop-lts") {
		t.Error("a catalog entry was taken for the user's own")
	}
}

func read(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(Path(dir))
	if err != nil {
		return ""
	}
	return string(data)
}
