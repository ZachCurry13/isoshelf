package web

import (
	"regexp"
	"strings"
	"testing"
)

// append and replaceChildren are not el. el skips a null child; they write it
// on the page as the word "null". That is how a lone filter chip came to be
// followed by "null" on the maintainer's phone in v0.5.1: the "Clear all"
// beside the chips was "chips.length > 1 ? link : null", handed straight to
// replaceChildren. Anything that may be nothing goes through el, or through
// .filter(Boolean) first.
func TestNothingHandsTheDOMANull(t *testing.T) {
	for _, name := range scripts {
		for _, line := range bareNulls(readStatic(t, name)) {
			t.Errorf("%s:%d passes null straight to append or replaceChildren, which "+
				"writes \"null\" on the page. Filter it out first.", name, line)
		}
	}
}

// The check has to catch the line that shipped, and leave alone the two ways
// of writing it that are fine.
func TestTheNullCheckCatchesWhatShipped(t *testing.T) {
	for _, c := range []struct {
		js   string
		want int
	}{
		{`box.replaceChildren(...chips, chips.length > 1 ? el("button", {}, "Clear all") : null);`, 1},
		{`box.replaceChildren(...[...chips, more ? el("button") : null].filter(Boolean));`, 0},
		{`box.append(el("div", {}, note ? el("p", {}, note) : null, "it's fine"));`, 0},
		{"box.append(`${a ? b : null}`, x ? y : null);", 1},
	} {
		if got := len(bareNulls(c.js)); got != c.want {
			t.Errorf("%s: found %d, want %d", c.js, got, c.want)
		}
	}
}

var (
	domCall  = regexp.MustCompile(`\.(append|prepend|replaceChildren|before|after|replaceWith)\(`)
	bareNull = regexp.MustCompile(`[?:]\s*null\b`)
)

// bareNulls returns the lines where a DOM call is handed null directly: as
// one of its own arguments, not inside a call or list within them.
func bareNulls(js string) []int {
	var lines []int
	for _, loc := range domCall.FindAllStringIndex(js, -1) {
		if bareNull.MatchString(ownArgs(js[loc[1]:])) {
			lines = append(lines, strings.Count(js[:loc[0]], "\n")+1)
		}
	}
	return lines
}

// ownArgs is the text of a call's arguments that belongs to the call itself,
// read from just after its opening bracket: strings, comments and anything in
// nested brackets are left out.
func ownArgs(s string) string {
	var b strings.Builder
	depth := 1
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		case c == '"' || c == '\'' || c == '`':
			quote = c
			continue
		case strings.IndexByte("([{", c) >= 0:
			depth++
			continue
		case strings.IndexByte(")]}", c) >= 0:
			depth--
			if depth == 0 {
				return b.String()
			}
			continue
		}
		if depth == 1 {
			b.WriteByte(c)
		}
	}
	return b.String()
}
