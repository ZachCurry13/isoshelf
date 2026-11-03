// Package primer builds static/css/primer.css, the part of Primer's design
// tokens (https://primer.style/product/primitives, MIT) that isoshelf uses.
//
// The tokens are Primer's, with their own names, so Primer's documentation
// describes them as written. Only the ones css/tokens.css refers to are kept
// (Primer's themes are about 120 KB each and this page uses about 50 names),
// so updating Primer is a version number and one command:
//
//	go run ./internal/web/primer/fetch
//
// This package only reads and filters CSS. It never goes online; the command
// beside it does, the same as the logo fetcher.
package primer

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Version is the @primer/primitives release the tokens come from. Change it,
// run the fetch command, read the diff.
const Version = "11.10.0"

// File is one source file in the package, relative to dist/css.
type File string

// URL is where the file is published.
func (f File) URL() string {
	return fmt.Sprintf("https://cdn.jsdelivr.net/npm/@primer/primitives@%s/dist/css/%s.css", Version, f)
}

// Theme is one of Primer's colour themes, kept under its own name. What
// Settings can ask for is every combination of three things: light or dark
// (the page needs both, to follow the computer), the colours (ordinary, or the
// two sets Primer makes for colour blindness), and ordinary or high contrast.
// Primer also has a dimmed theme; it is not offered.
type Theme struct {
	// Name is the value of data-light-theme or data-dark-theme.
	Name string
	File File
}

// Colours are the colour sets Settings offers, as Primer names them: the empty
// one is ordinary, "colorblind" is for red-green colour blindness (protanopia
// and deuteranopia) and "tritanopia" is for blue-yellow.
var Colours = []string{"", "colorblind", "tritanopia"}

// Themes are the colour themes shipped: twelve, from the combinations above.
// settings.js builds the same names from the same three choices.
var Themes = themes()

func themes() []Theme {
	var out []Theme
	for _, side := range []string{"light", "dark"} {
		for _, colours := range Colours {
			for _, contrast := range []bool{false, true} {
				name, file := side, "functional/themes/"+side
				if colours != "" {
					name, file = name+"_"+colours, file+"-"+colours
				}
				if contrast {
					name, file = name+"_high_contrast", file+"-high-contrast"
				}
				out = append(out, Theme{name, File(file)})
			}
		}
	}
	return out
}

// Base are the files of tokens that do not change with the theme: radii and
// type. They all declare on :root.
var Base = []File{
	"functional/size/radius",
	"functional/typography/typography",
	"base/typography/typography",
}

var (
	use  = regexp.MustCompile(`var\(\s*(--[A-Za-z0-9_-]+)`)
	decl = regexp.MustCompile(`^\s*(--[A-Za-z0-9_-]+)\s*:\s*(.*?)\s*;\s*(/\*.*)?$`)
	cmt  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	// define finds a custom property being set, wherever it is in a rule: the
	// hand-written stylesheets put several to a line, unlike Primer's.
	define = regexp.MustCompile(`(--[A-Za-z0-9_-]+)\s*:`)
)

// Defined returns the custom properties css declares.
func Defined(css string) map[string]bool {
	out := map[string]bool{}
	for _, m := range define.FindAllStringSubmatch(cmt.ReplaceAllString(css, ""), -1) {
		out[m[1]] = true
	}
	return out
}

// Used returns the custom properties css refers to with var().
func Used(css string) map[string]bool {
	out := map[string]bool{}
	for _, m := range use.FindAllStringSubmatch(cmt.ReplaceAllString(css, ""), -1) {
		out[m[1]] = true
	}
	return out
}

// Wanted returns the names css uses but does not define itself, which is to
// say the ones that have to come from Primer.
func Wanted(css string) []string {
	own := Defined(css)
	var out []string
	for name := range Used(css) {
		if !own[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Block is a rule: its selector (or @media line), the declarations in it and
// the rules nested inside it.
type Block struct {
	Head     string
	Decls    [][2]string
	Children []*Block
}

// Parse reads Primer's regular layout: one declaration to a line, blocks that
// open with "{" at the end of a line and close with "}" on a line of their own.
func Parse(src string) (*Block, error) {
	root := &Block{}
	stack := []*Block{root}
	// A selector list can run over several lines, each but the last ending in
	// a comma; pending holds the lines so far.
	pending := ""
	for n, line := range strings.Split(cmt.ReplaceAllString(src, ""), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
		case strings.HasSuffix(t, ","):
			pending += t + " "
		case strings.HasSuffix(t, "{"):
			b := &Block{Head: strings.TrimSpace(pending + strings.TrimSuffix(t, "{"))}
			pending = ""
			top := stack[len(stack)-1]
			top.Children = append(top.Children, b)
			stack = append(stack, b)
		case t == "}":
			if len(stack) == 1 {
				return nil, fmt.Errorf("line %d: a } with nothing open", n+1)
			}
			stack = stack[:len(stack)-1]
		default:
			m := decl.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("line %d: not a declaration: %q", n+1, t)
			}
			top := stack[len(stack)-1]
			top.Decls = append(top.Decls, [2]string{m[1], m[2]})
		}
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("a block is never closed")
	}
	return root, nil
}

// Close adds to wanted every token that a wanted token's value refers to, and
// so on, across all of the parsed files. Primer's type sizes, for one, are
// defined in terms of a scale defined in another file.
func Close(wanted []string, files ...*Block) []string {
	values := map[string][]string{} // every theme's value for a name: they can differ in kind
	var walk func(*Block)
	walk = func(b *Block) {
		for _, d := range b.Decls {
			values[d[0]] = append(values[d[0]], d[1])
		}
		for _, c := range b.Children {
			walk(c)
		}
	}
	for _, f := range files {
		walk(f)
	}
	have := map[string]bool{}
	queue := slices.Clone(wanted)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if have[name] {
			continue
		}
		have[name] = true
		for _, v := range values[name] {
			for _, m := range use.FindAllStringSubmatch(v, -1) {
				queue = append(queue, m[1])
			}
		}
	}
	out := make([]string, 0, len(have))
	for name := range have {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Filter renders b keeping only the declarations of the named tokens, and
// leaving out every block that ends up empty.
func Filter(b *Block, keep []string) string {
	want := map[string]bool{}
	for _, k := range keep {
		want[k] = true
	}
	var out strings.Builder
	// The root has no head: render its children at the top level.
	for _, c := range b.Children {
		renderTo(&out, c, "", want)
	}
	return out.String()
}

// renderTo writes block b to w, reporting whether it wrote anything.
func renderTo(w *strings.Builder, b *Block, indent string, want map[string]bool) bool {
	var body strings.Builder
	for _, d := range b.Decls {
		if want[d[0]] {
			fmt.Fprintf(&body, "%s  %s: %s;\n", indent, d[0], d[1])
		}
	}
	for _, c := range b.Children {
		renderTo(&body, c, indent+"  ", want)
	}
	if body.Len() == 0 {
		return false
	}
	fmt.Fprintf(w, "%s%s {\n%s%s}\n", indent, b.Head, body.String(), indent)
	return true
}
