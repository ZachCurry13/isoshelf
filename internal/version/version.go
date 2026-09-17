// Package version compares version strings the way people read them.
package version

import "strings"

// Compare returns -1, 0 or +1 depending on whether a is older than, the same
// as, or newer than b. Runs of digits compare as numbers and other runs as
// text, so "22.10" is newer than "22.4" and "260809" newer than "260308".
// Punctuation only separates parts, so "23.0.4" equals "23-0-4".
func Compare(a, b string) int {
	ta, tb := tokens(a), tokens(b)
	for i := range min(len(ta), len(tb)) {
		if c := compareToken(ta[i], tb[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(ta) < len(tb):
		return -1
	case len(ta) > len(tb):
		return 1
	}
	return 0
}

func compareToken(a, b string) int {
	aNum, bNum := isDigit(a[0]), isDigit(b[0])
	switch {
	case aNum && bNum:
		a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
		if len(a) != len(b) {
			if len(a) < len(b) {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	case aNum:
		return 1 // "1.0.1" is newer than "1.0.beta"
	case bNum:
		return -1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// tokens splits s into runs of digits and runs of letters, dropping
// everything else.
func tokens(s string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(s); i++ {
		if start >= 0 && (i == len(s) || !isAlnum(s[i]) || isDigit(s[i]) != isDigit(s[start])) {
			out = append(out, s[start:i])
			start = -1
		}
		if i < len(s) && start < 0 && isAlnum(s[i]) {
			start = i
		}
	}
	return out
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func isAlnum(c byte) bool {
	return isDigit(c) || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
