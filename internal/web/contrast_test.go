package web

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// contrastPairs are the words-on-a-background pairs the page draws, in
// isoshelf's own names from css/tokens.css, with the ratio WCAG asks of them:
// 4.5 for text (everything here is small text), 3 for an icon or the edge of a
// control. A background with transparency is laid over --bg first, as the
// browser does.
var contrastPairs = []struct {
	fg, bg string
	min    float64
}{
	{"--text", "--bg", 4.5},
	{"--text", "--surface-2", 4.5},
	{"--muted", "--bg", 4.5},
	{"--muted", "--surface-2", 4.5},
	{"--accent", "--bg", 4.5},
	{"--on-accent", "--accent-fill", 4.5},
	{"--primary-fg", "--primary-bg", 4.5},
	{"--header-fg", "--header-bg", 4.5},
	{"--header-fg-strong", "--header-bg", 4.5},
	{"--danger-fg", "--bg", 4.5},
	{"--ok-fg", "--ok-bg", 4.5},
	{"--update-fg", "--update-bg", 4.5},
	{"--bad-fg", "--bad-bg", 4.5},
	{"--eol-fg", "--eol-bg", 4.5},
	{"--warn-fg", "--warn-bg", 4.5},
	{"--missing-fg", "--missing-bg", 4.5},
	{"--manual-fg", "--manual-bg", 4.5},
	{"--muted-fg", "--muted-bg", 4.5},
	{"--ok-fg", "--bg", 4.5},
	{"--update-fg", "--bg", 4.5},
	{"--bad-fg", "--bg", 4.5},
	{"--eol-fg", "--bg", 4.5},
	{"--warn-fg", "--bg", 4.5},
	{"--star", "--bg", 3},
	{"--focus", "--bg", 3},
}

// TestContrastInEveryTheme measures every pair above in each of Primer's
// twelve themes (docs/primer-audit.md, F15), so a new Primer version or a
// changed token that makes something hard to read fails here rather than on
// somebody's screen.
func TestContrastInEveryTheme(t *testing.T) {
	tokens := tokenMap(t, "static/css/tokens.css", `:root {`)
	themes := primerThemes(t)
	if len(themes) != 12 {
		t.Fatalf("primer.css has %d themes, want 12", len(themes))
	}
	for name, vars := range themes {
		look := func(n string) rgba {
			v, ok := resolveVar(n, tokens, vars, 0)
			if !ok {
				t.Fatalf("%s: %s doesn't come to a colour", name, n)
			}
			c, err := parseColour(v)
			if err != nil {
				t.Fatalf("%s: %s: %v", name, n, err)
			}
			return c
		}
		page := look("--bg")
		for _, p := range contrastPairs {
			bg := look(p.bg).over(page)
			fg := look(p.fg).over(bg)
			if r := ratio(fg, bg); r < p.min {
				t.Errorf("%s: %s on %s is %.2f:1, want at least %.1f:1", name, p.fg, p.bg, r, p.min)
			}
		}
	}
}

var varRef = regexp.MustCompile(`^var\((--[\w-]+)\)$`)

// resolveVar follows var() from isoshelf's names into Primer's, using the
// theme's own value before the shared :root one.
func resolveVar(name string, tokens, theme map[string]string, depth int) (string, bool) {
	if depth > 10 {
		return "", false
	}
	v, ok := theme[name]
	if !ok {
		v, ok = tokens[name]
	}
	if !ok {
		return "", false
	}
	if m := varRef.FindStringSubmatch(v); m != nil {
		return resolveVar(m[1], tokens, theme, depth+1)
	}
	return v, true
}

var declRe = regexp.MustCompile(`(--[\w-]+)\s*:\s*([^;]+);`)

// tokenMap reads the custom properties declared in the block that opens with
// start.
func tokenMap(t *testing.T, path, start string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, start)
	if i < 0 {
		t.Fatalf("%s: no %q", path, start)
	}
	end := strings.Index(s[i:], "\n}")
	m := map[string]string{}
	for _, d := range declRe.FindAllStringSubmatch(s[i:i+end], -1) {
		m[d[1]] = strings.TrimSpace(d[2])
	}
	return m
}

var themeHead = regexp.MustCompile(`(?m)^\[data-color-mode="(?:light|dark)"\]\[data-(?:light|dark)-theme="(\w+)"\]`)

// primerThemes reads each theme's first block in primer.css (the one chosen
// by name; the media-query copy repeats it for "auto").
func primerThemes(t *testing.T) map[string]map[string]string {
	t.Helper()
	b, err := os.ReadFile("static/css/primer.css")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	out := map[string]map[string]string{}
	for _, loc := range themeHead.FindAllStringSubmatchIndex(s, -1) {
		name := s[loc[2]:loc[3]]
		end := strings.Index(s[loc[0]:], "\n}")
		vars := map[string]string{}
		for _, d := range declRe.FindAllStringSubmatch(s[loc[0]:loc[0]+end], -1) {
			vars[d[1]] = strings.TrimSpace(d[2])
		}
		out[name] = vars
	}
	return out
}

type rgba struct{ r, g, b, a float64 }

func parseColour(v string) (rgba, error) {
	h := strings.TrimPrefix(v, "#")
	if h == v || (len(h) != 6 && len(h) != 8) {
		return rgba{}, fmt.Errorf("%q isn't a #rrggbb or #rrggbbaa colour", v)
	}
	n, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return rgba{}, err
	}
	if len(h) == 6 {
		n = n<<8 | 0xff
	}
	return rgba{float64(n >> 24 & 0xff), float64(n >> 16 & 0xff), float64(n >> 8 & 0xff), float64(n&0xff) / 255}, nil
}

// over lays c on an opaque background.
func (c rgba) over(bg rgba) rgba {
	mix := func(f, b float64) float64 { return f*c.a + b*(1-c.a) }
	return rgba{mix(c.r, bg.r), mix(c.g, bg.g), mix(c.b, bg.b), 1}
}

func luminance(c rgba) float64 {
	lin := func(v float64) float64 {
		v /= 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

func ratio(a, b rgba) float64 {
	x, y := luminance(a), luminance(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}
