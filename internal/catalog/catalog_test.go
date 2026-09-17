package catalog

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

// validCatalog passes validation. It has one entry of each common shape: a
// versioned endoflife track with a signed manifest, a fixed-name GitHub
// asset, a manual entry, and a check-only listing without an artifact.
const validCatalog = `schema = 1

[keys.example]
file = "keys/example.asc"
fingerprints = ["0123 4567 89AB CDEF 0123  4567 89AB CDEF 0123 4567"]

[[entry]]
id = "example-x64"
name = "Example Linux"
arch = "x86_64"
match = 'example-(?P<version>\d+(?:\.\d+)*)-amd64\.iso'
samples = ["example-1.2-amd64.iso"]

[entry.source]
type = "endoflife"
product = "example"
channel = "latest"
cycles = '\d+(?:\.\d+)*'

[entry.artifact]
base = "https://example.org/releases/{cycle}/"
file = 'example-{version}-amd64\.iso'
manifest = "SHA256SUMS"
signature = "manifest"
sig = "SHA256SUMS.gpg"
key = "example"

[[entry]]
id = "example-tool"
name = "Example Tool"
arch = "multi"
match = 'example-tool\.iso'
fixed_name = true
samples = ["example-tool.iso"]

[entry.source]
type = "github"
repo = "example/tool"
tag = 'v(?P<version>\d+\.\d+\.\d+)'
asset = 'example-tool\.iso'

[[entry]]
id = "example-setup"
name = "Example Setup"
arch = "x86"
match = 'ExampleSetup_(?:(?P<version>\d+)_)?x86\.iso'
samples = ["ExampleSetup_x86.iso", "ExampleSetup_7_x86.iso"]
page = "https://example.org/download"
known_hashes = ["E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"]

[entry.source]
type = "manual"

[[entry]]
id = "example-beta"
name = "Example Beta"
arch = "arm64"
match = 'example-beta-(?P<version>\d+)-arm64\.iso'
samples = ["example-beta-7-arm64.iso"]
page = "https://example.org/beta"

[entry.source]
type = "listing"
url = "https://example.org/beta/"
regex = 'example-beta-(?P<version>\d+)-arm64\.iso'
`

// load reads a catalog from an in-memory folder that also holds the key file
// validCatalog refers to.
func load(t *testing.T, text string) (*Catalog, error) {
	t.Helper()
	fsys := fstest.MapFS{
		"catalog.toml":     {Data: []byte(text)},
		"keys/example.asc": {Data: []byte("not a real key")},
	}
	return Load(fsys, "catalog.toml")
}

func TestLoadValid(t *testing.T) {
	c, err := load(t, validCatalog)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		filename    string
		wantID      string // empty: no entry matches
		wantVersion string
	}{
		{"example-1.2-amd64.iso", "example-x64", "1.2"},
		{"example-24.04.3-amd64.iso", "example-x64", "24.04.3"},
		{"example-tool.iso", "example-tool", ""},
		{"ExampleSetup_x86.iso", "example-setup", ""},
		{"ExampleSetup_7_x86.iso", "example-setup", "7"},
		{"example-beta-12-arm64.iso", "example-beta", "12"},
		{"example-1.2-i386.iso", "", ""},
		{"old-example-tool.iso", "", ""}, // patterns match whole names only
		{"example-tool.iso.bak", "", ""},
	}
	for _, tt := range tests {
		matches := c.Match(tt.filename)
		if tt.wantID == "" {
			if len(matches) != 0 {
				t.Errorf("%s: matched %q, want no match", tt.filename, matches[0].Entry.ID)
			}
			continue
		}
		if len(matches) != 1 {
			t.Errorf("%s: %d matches, want 1", tt.filename, len(matches))
			continue
		}
		if got := matches[0]; got.Entry.ID != tt.wantID || got.Version != tt.wantVersion {
			t.Errorf("%s: matched %q version %q, want %q version %q",
				tt.filename, got.Entry.ID, got.Version, tt.wantID, tt.wantVersion)
		}
	}

	if e := c.Entry("example-tool"); e == nil || e.Name != "Example Tool" {
		t.Errorf("Entry(example-tool) = %v", e)
	}
	if e := c.Entry("nope"); e != nil {
		t.Errorf("Entry(nope) = %v, want nil", e)
	}
	if got, want := c.Keys["example"].Fingerprints[0], "0123456789ABCDEF0123456789ABCDEF01234567"; got != want {
		t.Errorf("fingerprint not normalized: got %q, want %q", got, want)
	}
	if got, want := c.Entry("example-setup").KnownHashes[0], "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"; got != want {
		t.Errorf("hash not normalized: got %q, want %q", got, want)
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name     string
		old, new string // validCatalog with old replaced by new
		want     string // text the error must contain
	}{
		// File level.
		{"missing schema", "schema = 1", "", "schema: missing"},
		{"newer schema", "schema = 1", "schema = 2", "reads schema 1"},
		{"misspelled key", `product = "example"`, `prodcut = "example"`, `unknown key "entry.source.prodcut"`},
		{"syntax error", `name = "Example Linux"`, `name = "Example Linux`, "line 9"},
		{"key file missing", `file = "keys/example.asc"`, `file = "keys/missing.asc"`, `cannot read "keys/missing.asc"`},
		{"key file outside catalog", `file = "keys/example.asc"`, `file = "../example.asc"`, "must be a relative path"},
		{"bad fingerprint", `"0123 4567 89AB CDEF 0123  4567 89AB CDEF 0123 4567"`, `"ABCD"`, `"ABCD" is not a 40 or 64 digit hex fingerprint`},

		// Identity.
		{"bad id", `id = "example-x64"`, `id = "Example_x64"`, `entry #1: id: "Example_x64" must be lowercase`},
		{"duplicate id", `id = "example-tool"`, `id = "example-x64"`, "id: also used by entry #1"},
		{"missing name", `name = "Example Tool"`, "", `entry "example-tool": name: required`},
		{"bad arch", `arch = "x86_64"`, `arch = "amd64"`, `arch: "amd64" is not one of x86_64, x86, arm64, arm, multi`},

		// Match patterns and samples.
		{"bad match regex", `match = 'example-tool\.iso'`, `match = 'example-tool(\.iso'`, "match: error parsing regexp"},
		{"no version group", `match = 'example-(?P<version>\d+(?:\.\d+)*)-amd64\.iso'`, `match = 'example-\d+(?:\.\d+)*-amd64\.iso'`, "needs a (?P<version>...) group"},
		{"fixed name with version", `match = 'example-tool\.iso'`, `match = 'example-tool(?P<version>\d*)\.iso'`, "fixed_name entries must not capture a version"},
		{"unknown group name", `(?P<version>\d+)_)?x86`, `(?P<ver>\d+)_)?x86`, `unknown group name "ver"`},
		{"no samples", `samples = ["example-tool.iso"]`, "", "add at least one real filename"},
		{"sample is a path", `samples = ["example-tool.iso"]`, `samples = ["iso/example-tool.iso"]`, "must be a bare filename"},
		{"sample does not match", `samples = ["example-1.2-amd64.iso"]`, `samples = ["example-1.2-arm64.iso"]`, `"example-1.2-arm64.iso" does not match this entry`},
		{"sample matches two entries", `match = 'example-tool\.iso'`, `match = 'example-.*\.iso'`, `"example-1.2-amd64.iso" also matches entry "example-tool"`},

		// Sources.
		{"unknown source type", `type = "manual"`, `type = "manul"`, `source.type: unknown type "manul"`},
		{"field of another source type", `type = "manual"`, "type = \"manual\"\nrepo = \"example/tool\"", "source.repo: not used by manual sources"},
		{"missing channel", `channel = "latest"`, "", "source.channel: required for endoflife sources"},
		{"bad channel", `channel = "latest"`, `channel = "latest please"`, `must be "latest", "lts" or a release cycle`},
		{"bad cycles regex", `cycles = '\d+(?:\.\d+)*'`, `cycles = '\d+(('`, "source.cycles: error parsing regexp"},
		{"cycles on another source type", `type = "manual"`, "type = \"manual\"\ncycles = '1'", "source.cycles: not used by manual sources"},
		{"bad repo", `repo = "example/tool"`, `repo = "example"`, "must look like owner/name"},
		{"bad tag regex", `tag = 'v(?P<version>\d+\.\d+\.\d+)'`, `tag = 'v(\d+'`, "source.tag: error parsing regexp"},
		{"listing regex without version", "type = \"github\"\nrepo = \"example/tool\"\ntag = 'v(?P<version>\\d+\\.\\d+\\.\\d+)'\nasset = 'example-tool\\.iso'", "type = \"listing\"\nurl = \"https://example.org/tool/\"\nregex = 'tool v\\d+'", "source.regex: needs a (?P<version>...) group"},

		// Manual entries and extras.
		{"manual without page", `page = "https://example.org/download"`, "", "page: required for manual entries"},
		{"manual with artifact", `type = "manual"`, "type = \"manual\"\n\n[entry.artifact]\nbase = \"https://example.org/\"\nfile = 'x\\.iso'", "manual entries have no artifact"},
		{"bad fixup", `fixed_name = true`, "fixed_name = true\nfixup = \"rename:.bin\"", `rename target ".bin" is not a type Ventoy lists`},
		{"bad known hash", `"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"`, `"abc"`, `"abc" is not a SHA-256`},
		{"known hash in two entries", `fixed_name = true`, "fixed_name = true\nknown_hashes = [\"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\"]", `already listed by entry "example-tool"`},

		// Artifacts.
		{"github asset with artifact", `asset = 'example-tool\.iso'`, "asset = 'example-tool\\.iso'\n\n[entry.artifact]\nbase = \"https://example.org/\"\nfile = 'x\\.iso'", "not used when source.asset is set"},
		{"check-only entry without page", `page = "https://example.org/beta"`, "", "page: required for entries without [entry.artifact]"},
		{"github label without artifact or page", `asset = 'example-tool\.iso'`, "", "page: required for entries without [entry.artifact]"},
		{"fixed name without checksum", `asset = 'example-tool\.iso'`, "", "fixed_name: needs a published checksum"},
		{"http base", `base = "https://example.org/releases/{cycle}/"`, `base = "http://example.org/releases/{cycle}/"`, "must be an https:// URL"},
		{"base without slash", `base = "https://example.org/releases/{cycle}/"`, `base = "https://example.org/releases/{cycle}"`, "artifact.base: must end with /"},
		{"placeholder from another source", `base = "https://example.org/releases/{cycle}/"`, `base = "https://example.org/releases/{tag}/"`, "{tag} cannot be used here; available: {cycle}, {version}"},
		{"file placeholder in base", `base = "https://example.org/releases/{cycle}/"`, `base = "https://example.org/releases/{file}/"`, "{file} cannot be used here"},
		{"misspelled placeholder", `file = 'example-{version}-amd64\.iso'`, `file = 'example-{vesion}-amd64\.iso'`, "{vesion} cannot be used here"},
		{"bad file regex", `file = 'example-{version}-amd64\.iso'`, `file = 'example-{version}-(amd64\.iso'`, "artifact.file: error parsing regexp"},
		{"http manifest", `manifest = "SHA256SUMS"`, `manifest = "http://example.org/SHA256SUMS"`, "must be relative to artifact.base or an https:// URL"},
		{"mirrors without manifest", `manifest = "SHA256SUMS"`, `mirrors = ["https://mirror.example.net/example/{cycle}/"]`, "mirrors need artifact.manifest"},
		{"unknown signature", `signature = "manifest"`, `signature = "detached"`, `"detached" is not "manifest", "clearsigned" or "image"`},
		{"clearsigned with sig", `signature = "manifest"`, `signature = "clearsigned"`, `artifact.sig: not used when signature = "clearsigned"`},
		{"sig without signature", `signature = "manifest"`, "", "set artifact.signature"},
		{"missing key table", `key = "example"`, `key = "other"`, "no [keys.other] table"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if n := strings.Count(validCatalog, tt.old); n != 1 {
				t.Fatalf("test setup: %q appears %d times in validCatalog, want 1", tt.old, n)
			}
			_, err := load(t, strings.Replace(validCatalog, tt.old, tt.new, 1))
			if err == nil {
				t.Fatal("got no error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error does not contain %q:\n%v", tt.want, err)
			}
		})
	}
}

func TestLoadReportsAllProblems(t *testing.T) {
	text := strings.Replace(validCatalog, `arch = "x86_64"`, `arch = "amd64"`, 1)
	text = strings.Replace(text, `channel = "latest"`, `channel = ""`, 1)
	_, err := load(t, text)

	var cerr *Error
	if !errors.As(err, &cerr) {
		t.Fatalf("want *Error, got %v", err)
	}
	if len(cerr.Problems) != 2 {
		t.Errorf("got %d problems, want 2:\n%v", len(cerr.Problems), err)
	}
	for _, p := range cerr.Problems {
		if p.Entry != "example-x64" || p.Index != 1 {
			t.Errorf("problem %q is not attributed to entry example-x64 (#1)", p)
		}
	}
}

func TestLoadUnknownKeyLine(t *testing.T) {
	text := strings.Replace(validCatalog, `product = "example"`, `prodcut = "example"`, 1)
	_, err := load(t, text)

	var cerr *Error
	if !errors.As(err, &cerr) || len(cerr.Problems) != 1 {
		t.Fatalf("want one problem, got %v", err)
	}
	wantLine := 1 + strings.Count(text[:strings.Index(text, "prodcut")], "\n")
	if got := cerr.Problems[0].Line; got != wantLine {
		t.Errorf("line = %d, want %d", got, wantLine)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(fstest.MapFS{}, "catalog.toml"); err == nil {
		t.Error("want an error for a missing file")
	}
}
