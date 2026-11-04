package web

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Primer's spacing scale, in pixels (foundations/primitives/size). Every
// padding, margin and gap in the stylesheets is one of these, through a
// --sp-N token, so that the page keeps one rhythm and a later change to the
// scale is a change to tokens.css (docs/design-system.md).
var spaceScale = []int{2, 4, 6, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 64}

var spacingDecl = regexp.MustCompile(`(?:^|[;{\s])((?:padding|margin|gap|row-gap|column-gap)(?:-(?:top|right|bottom|left|block|inline)(?:-(?:start|end))?)?)\s*:\s*([^;{}]+)[;}]`)

func TestSpacingIsOnPrimersScale(t *testing.T) {
	for name, css := range handWritten(t) {
		if name == "tokens.css" {
			continue
		}
		for _, m := range spacingDecl.FindAllStringSubmatch(css, -1) {
			for _, word := range strings.Fields(m[2]) {
				if word == "0" || word == "auto" || word == "!important" ||
					strings.HasPrefix(word, "var(--sp-") || strings.HasPrefix(word, "calc(") ||
					strings.HasPrefix(word, "-") && strings.Contains(word, "var(--sp-") {
					continue
				}
				if strings.HasSuffix(word, "%") || strings.HasPrefix(word, "var(") || word == "*" || word == "+" || word == ")" {
					continue
				}
				t.Errorf("%s: %s: %s uses %q, which is not a Primer spacing token (use var(--sp-N); the scale is %v)",
					name, m[1], strings.TrimSpace(m[2]), word, spaceScale)
			}
		}
	}
}

// The tokens themselves are the scale and nothing else.
func TestSpacingTokensAreThePrimerScale(t *testing.T) {
	tokens := cssFiles(t)["tokens.css"]
	found := map[int]bool{}
	for _, m := range regexp.MustCompile(`--sp-(\d+):\s*calc\((\d+)rem / 14\)`).FindAllStringSubmatch(tokens, -1) {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		if a != b {
			t.Errorf("--sp-%d is %d px", a, b)
		}
		found[a] = true
	}
	for _, n := range spaceScale {
		if !found[n] {
			t.Errorf("tokens.css has no --sp-%d", n)
		}
	}
	if len(found) != len(spaceScale) {
		t.Errorf("tokens.css has %d spacing tokens, Primer's scale has %d", len(found), len(spaceScale))
	}
}
