// Package usercat holds the images a user names themselves: files no catalog
// will ever know, such as a Windows installer they built, a customized image,
// or a distribution isoshelf hasn't been told about yet.
//
// These live in their own file, <config>/catalog-mine.toml, and are merged
// onto whichever catalog isoshelf is using. Keeping them separate is what
// lets the published catalog carry on updating itself underneath them.
//
// An entry named this way is inventory only: it gives the file a name, a kind
// and a link. isoshelf never invents a download address, because an address
// nobody checked is exactly how the wrong image gets trusted.
package usercat

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
)

// FileName is the file in the config folder. Users can edit it by hand: it is
// an ordinary catalog, and isoshelf only ever appends to it.
const FileName = "catalog-mine.toml"

// idPrefix marks entries as the user's own, and keeps their ids from ever
// colliding with the published catalog's.
const idPrefix = "my-"

const header = `# Images you named yourself.
#
# isoshelf appends to this file and never rewrites it, so you can edit it.
# Each [[entry]] works exactly like one in isoshelf's own catalog: see
# https://github.com/ZachCurry13/isoshelf for what the fields mean.
#
# These entries are inventory only. To have isoshelf download and check an
# image for you, it needs somewhere official to read checksums from, which is
# what the published catalog is for.

schema = 1
`

// Image is what the user says a file is.
type Image struct {
	// Name is what to call it, such as "Windows Server 2022".
	Name string
	// Filename is the file being named. Only this exact name matches.
	Filename string
	// Arch is one of catalog.Arches; empty means x86_64.
	Arch string
	// Category is one of catalog.Categories; empty means "other".
	Category string
	// Page is where the image can be downloaded again, if the user knows.
	Page string
}

// Path returns the file entries are kept in.
func Path(configDir string) string {
	return filepath.Join(configDir, FileName)
}

// Load returns the user's own entries, or nil when they have none.
func Load(configDir string) (*catalog.Catalog, error) {
	if configDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(Path(configDir)); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return catalog.Load(os.DirFS(configDir), FileName)
}

// Add writes one image into the user's file and returns the file's entries as
// they stand afterwards. The file is only kept if what it holds still loads
// and validates against base, so a name that clashes with an existing image
// leaves nothing behind.
func Add(configDir string, base *catalog.Catalog, img Image) (*catalog.Catalog, error) {
	if configDir == "" {
		return nil, errors.New("there is nowhere to keep your own images")
	}
	block, err := format(img)
	if err != nil {
		return nil, err
	}
	// Catch the common mistake before writing anything, so the message is
	// about the name rather than about catalog ids.
	id := ID(img.Name)
	if base != nil && base.Entry(id) != nil {
		return nil, fmt.Errorf("there is already an image called %q", strings.TrimSpace(img.Name))
	}

	name := Path(configDir)
	before, err := os.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if len(before) == 0 {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, err
		}
		before = []byte(header)
	}
	after := append(slices.Clone(before), block...)
	if err := os.WriteFile(name, after, 0o644); err != nil {
		return nil, err
	}

	mine, err := Load(configDir)
	if err == nil {
		_, err = catalog.Merge(base, mine)
	}
	if err != nil {
		// Put the file back the way it was: a half-written catalog would
		// follow the user around every time isoshelf starts.
		if len(before) == len(header) {
			os.Remove(name)
		} else {
			os.WriteFile(name, before, 0o644)
		}
		return nil, fmt.Errorf("%s couldn't be added: %w", img.Name, err)
	}
	return mine, nil
}

// format writes one entry as TOML. Only the fields a person can fill in are
// written, so the file stays readable and easy to edit by hand.
func format(img Image) (string, error) {
	name := strings.TrimSpace(img.Name)
	file := strings.TrimSpace(img.Filename)
	switch {
	case name == "":
		return "", errors.New("give the image a name")
	case file == "":
		return "", errors.New("there is no file to name")
	case strings.ContainsAny(file, `/\`):
		return "", errors.New("name a file in the folder, not a path")
	}
	arch := img.Arch
	if arch == "" {
		arch = "x86_64"
	}
	if !slices.Contains(catalog.Arches, arch) {
		return "", fmt.Errorf("%q isn't an architecture isoshelf knows", arch)
	}
	category := img.Category
	if category != "" && !slices.Contains(catalog.Categories, category) {
		return "", fmt.Errorf("%q isn't a kind isoshelf knows", category)
	}
	page := strings.TrimSpace(img.Page)
	if page != "" && !strings.HasPrefix(page, "https://") && !strings.HasPrefix(page, "http://") {
		return "", errors.New("a link has to start with https://")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n[[entry]]\nid = %q\nname = %q\narch = %q\n", ID(name), name, arch)
	// Only this exact filename, escaped, so naming one file never claims
	// another. isoshelf matches the pattern against the whole name.
	fmt.Fprintf(&b, "match = '%s'\n", regexp.QuoteMeta(file))
	fmt.Fprintf(&b, "samples = [%q]\n", file)
	if category != "" {
		fmt.Fprintf(&b, "category = %q\n", category)
	}
	if page != "" {
		fmt.Fprintf(&b, "page = %q\n", page)
	}
	b.WriteString("[entry.source]\ntype = \"manual\"\n")
	return b.String(), nil
}

var notID = regexp.MustCompile(`[^a-z0-9]+`)

// ID turns a name into a catalog id of the user's own: lowercase, digits and
// single dashes, always starting with "my-".
func ID(name string) string {
	id := notID.ReplaceAllString(strings.ToLower(name), "-")
	id = strings.Trim(id, "-")
	if id == "" {
		id = "image"
	}
	return idPrefix + id
}

// Mine reports whether an entry is one the user named themselves.
func Mine(id string) bool { return strings.HasPrefix(id, idPrefix) }
