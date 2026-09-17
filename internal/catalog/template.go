package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

// Templates are strings with {name} placeholders, such as
// "https://example.org/{cycle}/". Regex quantifiers like {2} or {2,4} start
// with a digit, so they are never mistaken for placeholders.
var placeholderRE = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Placeholders returns the placeholder names used in tmpl, in order of first
// use.
func Placeholders(tmpl string) []string {
	var names []string
	seen := map[string]bool{}
	for _, m := range placeholders(tmpl) {
		name := tmpl[m[2]:m[3]]
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// Expand replaces every {name} in tmpl with vars[name]. It fails if a
// placeholder has no value.
func Expand(tmpl string, vars map[string]string) (string, error) {
	return expand(tmpl, vars, func(s string) string { return s })
}

// ExpandRegexp expands a regex template, escaping each value so that it only
// matches itself (a version "22.3" must not match "2203"), and compiles the
// result to match whole strings only.
func ExpandRegexp(tmpl string, vars map[string]string) (*regexp.Regexp, error) {
	expr, err := expand(tmpl, vars, regexp.QuoteMeta)
	if err != nil {
		return nil, err
	}
	return WholeRegexp(expr)
}

func expand(tmpl string, vars map[string]string, quote func(string) string) (string, error) {
	var b strings.Builder
	last := 0
	for _, m := range placeholders(tmpl) {
		name := tmpl[m[2]:m[3]]
		value, ok := vars[name]
		if !ok {
			return "", fmt.Errorf("no value for {%s}", name)
		}
		b.WriteString(tmpl[last:m[0]])
		b.WriteString(quote(value))
		last = m[1]
	}
	b.WriteString(tmpl[last:])
	return b.String(), nil
}

// placeholders returns the submatch indexes of each placeholder in tmpl. It
// skips regex Unicode classes such as \p{Greek}.
func placeholders(tmpl string) [][]int {
	var out [][]int
	for _, m := range placeholderRE.FindAllStringSubmatchIndex(tmpl, -1) {
		if m[0] >= 2 && tmpl[m[0]-2] == '\\' && (tmpl[m[0]-1] == 'p' || tmpl[m[0]-1] == 'P') {
			continue
		}
		out = append(out, m)
	}
	return out
}

// WholeRegexp compiles expr so that it only matches an entire string, the way
// catalog patterns match. The unwrapped expression is compiled first so error
// messages show what the catalog author wrote.
func WholeRegexp(expr string) (*regexp.Regexp, error) {
	if _, err := regexp.Compile(expr); err != nil {
		return nil, err
	}
	return regexp.Compile(`^(?:` + expr + `)$`)
}
