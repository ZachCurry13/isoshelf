// Package octicons builds static/octicons.svg, the icons the page draws, from
// GitHub's Octicons (https://primer.style/octicons, MIT, copyright GitHub, Inc.;
// the licence is in this folder and is kept with the sprite).
//
// Primer's guidance (foundations/icons) is followed as written: icons are used
// at the size they are drawn, 16 px by default and 12 px only in condensed
// spots, and they take the colour of the text around them. A name here is the
// Octicons name with its size, as in the sprite's symbol ids ("gear-16").
//
// Updating is a version number and one command, the same as the Primer tokens
// and the logos:
//
//	go run ./internal/web/octicons/fetch
//
// This package only reads and builds. It never goes online; the command beside
// it does.
package octicons

import (
	"fmt"
	"regexp"
	"strings"
)

// Version is the @primer/octicons release the icons come from.
const Version = "19.42.0"

// Names is every icon the page may draw. The page's scripts and stylesheets
// are checked against it (octicons_test.go in internal/web), so an icon that
// isn't here can't be asked for, and one nothing asks for is a line to delete.
var Names = []string{
	// 16 px, the default.
	"alert-16", "arrow-down-16", "arrow-up-16", "bookmark-16", "bookmark-fill-16",
	"check-16", "check-circle-16", "chevron-right-16", "download-16", "filter-16",
	"gear-16", "grabber-16", "info-16", "move-to-top-16", "pin-16", "question-16",
	"star-16", "star-fill-16", "stop-16", "sync-16", "x-16",
	// 12 px, for condensed places only.
	"alert-fill-12",
}

// SourceURL is where an icon is published.
func SourceURL(name string) string {
	return fmt.Sprintf("https://cdn.jsdelivr.net/npm/@primer/octicons@%s/build/svg/%s.svg", Version, name)
}

// LicenseURL is where the licence is published.
func LicenseURL() string {
	return fmt.Sprintf("https://cdn.jsdelivr.net/npm/@primer/octicons@%s/LICENSE", Version)
}

var (
	viewBox = regexp.MustCompile(`viewBox="([^"]+)"`)
	inner   = regexp.MustCompile(`(?s)<svg[^>]*>(.*)</svg>`)
)

// Symbol turns one published icon into a <symbol> for the sprite.
func Symbol(name, svg string) (string, error) {
	vb := viewBox.FindStringSubmatch(svg)
	body := inner.FindStringSubmatch(svg)
	if vb == nil || body == nil {
		return "", fmt.Errorf("%s: not an SVG icon", name)
	}
	return fmt.Sprintf(`<symbol id="%s" viewBox="%s">%s</symbol>`, name, vb[1], strings.TrimSpace(body[1])), nil
}

// Sprite joins symbols into the file the page draws from.
func Sprite(symbols []string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg">` + "\n" +
		strings.Join(symbols, "\n") + "\n</svg>\n"
}

// Size is the pixel size an icon is drawn at, from the end of its name.
func Size(name string) int {
	if strings.HasSuffix(name, "-12") {
		return 12
	}
	if strings.HasSuffix(name, "-16") {
		return 16
	}
	return 0
}
