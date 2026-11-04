package web

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/web/octicons"
)

// Icons are Octicons, used at the size they are drawn (docs/design-system.md,
// "Icons"). The page asks for them by name in its scripts and its HTML; this
// holds the three to each other: what is asked for exists, what exists is
// asked for, and the licence travels with the icons.

var iconName = regexp.MustCompile(`"([a-z]+(?:-[a-z]+)*-(?:12|16))"`)
var iconRef = regexp.MustCompile(`octicons\.svg#([a-z0-9-]+)`)

func iconsAskedFor(t *testing.T) map[string]string {
	t.Helper()
	asked := map[string]string{}
	for _, name := range scripts {
		for _, m := range iconName.FindAllStringSubmatch(readStatic(t, name), -1) {
			asked[m[1]] = name
		}
	}
	for _, m := range iconRef.FindAllStringSubmatch(readStatic(t, "index.html"), -1) {
		asked[m[1]] = "index.html"
	}
	for _, m := range iconRef.FindAllStringSubmatch(readStatic(t, "styleguide.html"), -1) {
		asked[m[1]] = "styleguide.html"
	}
	return asked
}

func TestEveryIconAskedForExists(t *testing.T) {
	for name, where := range iconsAskedFor(t) {
		if !slices.Contains(octicons.Names, name) {
			t.Errorf("%s asks for the icon %q, which is not in octicons.Names (add it there and run go run ./internal/web/octicons/fetch)", where, name)
		}
	}
}

func TestEveryIconInTheSpriteIsUsed(t *testing.T) {
	asked := iconsAskedFor(t)
	for _, name := range octicons.Names {
		if _, ok := asked[name]; !ok {
			t.Errorf("octicons.Names lists %q but nothing uses it: delete it and rebuild the sprite", name)
		}
	}
}

func TestSpriteHasExactlyTheNamedIcons(t *testing.T) {
	sprite := readStatic(t, "octicons.svg")
	var ids []string
	for _, m := range regexp.MustCompile(`<symbol id="([^"]+)" viewBox="0 0 (\d+) \d+"`).FindAllStringSubmatch(sprite, -1) {
		ids = append(ids, m[1])
		if want := octicons.Size(m[1]); m[2] != itoa(want) {
			t.Errorf("%s is drawn on a %s px grid but its name says %d", m[1], m[2], want)
		}
	}
	slices.Sort(ids)
	want := slices.Clone(octicons.Names)
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("the sprite and octicons.Names differ; run go run ./internal/web/octicons/fetch\n sprite: %v\n names:  %v", ids, want)
	}
}

// Primer allows 12 px icons in condensed places only, and no other size.
func TestIconSizesAreOcticonSizes(t *testing.T) {
	for _, name := range octicons.Names {
		if s := octicons.Size(name); s != 12 && s != 16 {
			t.Errorf("%s has no Octicon size", name)
		}
	}
}

func TestOcticonsLicenceIsKept(t *testing.T) {
	b, err := os.ReadFile("octicons/LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "MIT License") || !strings.Contains(string(b), "GitHub") {
		t.Error("octicons/LICENSE is not GitHub's MIT licence")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
