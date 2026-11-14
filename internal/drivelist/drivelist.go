// Package drivelist is a drive's list of images as a small file a person
// keeps (#11, #12): what to fetch again when a drive is lost, or to fill a
// new one. It names images by their catalog id and nothing else that
// matters - no files are copied and nothing of the drive is - because
// everything on it is still on its project's own server.
package drivelist

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// List is one drive's images.
type List struct {
	// Folder is where the list was made from, and Made when; both only for
	// the person reading it.
	Folder string    `toml:"folder,omitempty"`
	Made   time.Time `toml:"made,omitzero"`
	Images []Image   `toml:"image"`
}

// Image is one image on the list. Only ID is needed to fetch it again; the
// rest is so the file makes sense to read, and so a star comes back.
type Image struct {
	ID      string `toml:"id"`
	Name    string `toml:"name,omitempty"`
	Version string `toml:"version,omitempty"`
	File    string `toml:"file,omitempty"`
	Starred bool   `toml:"starred,omitempty"`
}

const header = `# isoshelf drive list: the images a drive carries, by name.
#
# Keep it somewhere other than the drive. If the drive is lost, or you move
# to a bigger one, open isoshelf on the new folder, go to Settings, This
# folder, and choose "Fill from a list": every image here that isoshelf can
# download comes back from its project's own site, checked against the
# project's checksum. Files that aren't on this list - your own files,
# persistence files, anything isoshelf doesn't know - aren't on it because
# isoshelf can't get them back.

`

// Encode writes the list as the file a person keeps.
func Encode(l List) ([]byte, error) {
	slices.SortFunc(l.Images, func(a, b Image) int { return strings.Compare(a.ID, b.ID) })
	body, err := toml.Marshal(l)
	if err != nil {
		return nil, err
	}
	return append([]byte(header), body...), nil
}

// Decode reads a list file. It is lenient about what it doesn't need, since
// people edit these by hand; an image without an id is the one thing it
// refuses.
func Decode(data []byte) (List, error) {
	var l List
	if err := toml.NewDecoder(bytes.NewReader(data)).Decode(&l); err != nil {
		return List{}, fmt.Errorf("that isn't an isoshelf drive list: %w", err)
	}
	seen := map[string]bool{}
	var out []Image
	for i, img := range l.Images {
		if img.ID == "" {
			return List{}, fmt.Errorf("image %d on the list has no id", i+1)
		}
		if !seen[img.ID] {
			seen[img.ID] = true
			out = append(out, img)
		}
	}
	if len(out) == 0 {
		return List{}, fmt.Errorf("the list names no images")
	}
	l.Images = out
	return l, nil
}
