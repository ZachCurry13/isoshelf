package web

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/web/primer"
)

// The rules that keep the look in one place (docs/design-system.md). They are
// tests rather than advice because the way this goes wrong is quiet: one
// colour typed into one rule works, looks fine in light, and is wrong in dark
// and in high contrast, and nobody sees it until someone who needs those does.

// handWritten are the stylesheets a person edits; primer.css is generated.
func handWritten(t *testing.T) map[string]string {
	t.Helper()
	out := cssFiles(t)
	delete(out, "primer.css")
	return out
}

// Names that are set from outside the stylesheets, one per element, by the
// scripts: a brand colour and a logo for a tile, and how tall the downloads
// bar is. Everything else must be
// defined in a stylesheet.
var setByScripts = map[string]bool{"--brand": true, "--logo": true, "--dock-height": true}

// Every name a rule uses has to be defined somewhere, or the rule quietly does
// nothing. This is how a misspelt token, or one Primer renamed, shows up.
func TestEveryTokenUsedIsDefined(t *testing.T) {
	all := allCSS(t)
	defined := primer.Defined(all)
	var missing []string
	for name := range primer.Used(all) {
		if !defined[name] && !setByScripts[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("used but never defined: %s\n"+
			"If it is a Primer name, add it to css/tokens.css and run go run ./internal/web/primer/fetch.",
			strings.Join(missing, ", "))
	}
}

// primer.css has a block per theme, so a name can be defined in light and
// missing in dark. Each of the twelve themes must define every Primer name the
// page uses, or one of them (Higher contrast, say) loses a colour.
func TestEveryThemeDefinesEveryPrimerName(t *testing.T) {
	src := readStatic(t, "css/primer.css")
	root, err := primer.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var base, themeNames = map[string]bool{}, map[string]map[string]bool{}
	for _, th := range primer.Themes {
		themeNames[th.Name] = map[string]bool{}
	}
	var walk func(b *primer.Block)
	walk = func(b *primer.Block) {
		for _, th := range primer.Themes {
			if strings.Contains(b.Head, `-theme="`+th.Name+`"`) {
				for _, d := range b.Decls {
					themeNames[th.Name][d[0]] = true
				}
			}
		}
		if b.Head == ":root" {
			for _, d := range b.Decls {
				base[d[0]] = true
			}
		}
		for _, c := range b.Children {
			walk(c)
		}
	}
	walk(root)

	need := map[string]bool{}
	for _, n := range primer.Wanted(readStatic(t, "css/tokens.css")) {
		need[n] = true
	}
	for n := range primer.Used(src) {
		need[n] = true
	}
	for _, th := range primer.Themes {
		var missing []string
		for n := range need {
			if !base[n] && !themeNames[th.Name][n] {
				missing = append(missing, n)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("theme %s doesn't define: %s\nRun go run ./internal/web/primer/fetch.", th.Name, strings.Join(missing, ", "))
		}
	}
}

// primer.css says which version it is from; it must be the pinned one, or
// someone changed one without running the command.
func TestPrimerFileMatchesThePinnedVersion(t *testing.T) {
	src := readStatic(t, "css/primer.css")
	if !strings.Contains(src, "@primer/primitives "+primer.Version+" ") {
		t.Errorf("css/primer.css isn't from @primer/primitives %s. Run go run ./internal/web/primer/fetch.", primer.Version)
	}
}

var rawColour = regexp.MustCompile(`(?i):[^;{}]*(#[0-9a-f]{3,8}\b|\brgba?\(|\bhsla?\(|\blight-dark\(|\b(white|black)\b)`)
var comment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// A colour typed into a rule is a colour the themes can't change. Rules use
// the names in tokens.css; tokens.css alone says what they are.
func TestNoRawColoursOutsideTokens(t *testing.T) {
	for name, css := range handWritten(t) {
		if name == "tokens.css" {
			continue
		}
		for i, line := range strings.Split(comment.ReplaceAllString(css, ""), "\n") {
			if rawColour.MatchString(line) {
				t.Errorf("css/%s: a colour written into a rule (line %d of the file without its comments):\n  %s\n"+
					"Use a name from css/tokens.css; if none fits, add one there.", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// Primer's own names stay inside tokens.css, so replacing Primer with another
// system is a change to that one file.
func TestOnlyTokensCssUsesPrimerNames(t *testing.T) {
	own := primer.Defined(strings.Join(valuesOf(handWritten(t)), "\n"))
	tokens := readStatic(t, "css/tokens.css")
	fromPrimer := map[string]bool{}
	for _, n := range primer.Wanted(tokens) {
		fromPrimer[n] = true
	}
	for name, css := range handWritten(t) {
		if name == "tokens.css" {
			continue
		}
		for n := range primer.Used(css) {
			if fromPrimer[n] && !own[n] {
				t.Errorf("css/%s uses %s, a Primer name. Use isoshelf's own name for it from css/tokens.css.", name, n)
			}
		}
	}
}

func valuesOf(m map[string]string) []string {
	var out []string
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// index.html has to link exactly the stylesheets that exist, with Primer's
// first and the names that use it second: a file nobody links does nothing,
// and a link to a file that is gone is a 404 on every page load.
func TestIndexLinksEveryStylesheet(t *testing.T) {
	page := readStatic(t, "index.html")
	linked := regexp.MustCompile(`href="/static/css/([^"]+\.css)"`).FindAllStringSubmatch(page, -1)
	var order []string
	seen := map[string]bool{}
	for _, m := range linked {
		order = append(order, m[1])
		seen[m[1]] = true
	}
	if len(order) < 2 || order[0] != "primer.css" || order[1] != "tokens.css" {
		t.Errorf("index.html should link primer.css, then tokens.css, first; it links %v", order)
	}
	for name := range cssFiles(t) {
		if !seen[name] {
			t.Errorf("css/%s exists but index.html doesn't link it", name)
		}
	}
	for _, name := range order {
		if _, ok := cssFiles(t)[name]; !ok {
			t.Errorf("index.html links css/%s, which doesn't exist", name)
		}
	}
}

// The same rule the rest of the repository has: split a file by what it does
// before it gets hard to read.
func TestStylesheetsStayShort(t *testing.T) {
	for name, css := range handWritten(t) {
		if n := strings.Count(css, "\n"); n > 200 {
			t.Errorf("css/%s is %d lines; split it by what it styles (see CLAUDE.md)", name, n)
		}
	}
}

func linkedStylesheets(page string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`href="/static/css/([^"]+\.css)"`).FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

// The style guide is where a change to the design is looked at, so it has to
// be made of the same files as the page, in the same order; one that links a
// different set shows a look nobody has.
func TestStyleGuideLinksTheSameStylesheetsAsThePage(t *testing.T) {
	page := linkedStylesheets(readStatic(t, "index.html"))
	guide := linkedStylesheets(readStatic(t, "styleguide.html"))
	if strings.Join(page, " ") != strings.Join(guide, " ") {
		t.Errorf("styleguide.html links\n  %v\nbut index.html links\n  %v", guide, page)
	}
}

// Every stylesheet has a section in the guide (data-css="name"), so a new one
// can't go in without somewhere to see it.
func TestStyleGuideShowsEveryStylesheet(t *testing.T) {
	guide := readStatic(t, "styleguide.html")
	for name := range handWritten(t) {
		section := `data-css="` + strings.TrimSuffix(name, ".css") + `"`
		if !strings.Contains(guide, section) {
			t.Errorf("styleguide.html has no section for css/%s: add one marked %s", name, section)
		}
	}
}

// The guide has no JavaScript engine in the tests either, so it gets the same
// first-line and brace checks as the page's own scripts.
func TestStyleGuideScriptIsWellFormed(t *testing.T) {
	js := readStatic(t, "styleguide.js")
	if !strings.HasPrefix(js, `"use strict";`) {
		t.Error(`styleguide.js should start with "use strict"`)
	}
	if n := strings.Count(js, "{") - strings.Count(js, "}"); n != 0 {
		t.Errorf("styleguide.js has %d more { than }", n)
	}
	if strings.Contains(js, "innerHTML") {
		t.Error("styleguide.js uses innerHTML; text goes in with textContent, as everywhere else")
	}
}
