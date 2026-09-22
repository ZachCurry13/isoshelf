// Package peer talks to another isoshelf on the same network: the one on the
// machine the images already live on, usually a NAS.
//
// The point is not to avoid the internet but to avoid crossing it twice. A
// NAS that already holds a 6 GB image and a laptop about to fetch the same
// one from the other side of the world is a silly way round, and the local
// copy arrives in a minute rather than an hour.
//
// What makes it safe to prefer is that nothing about verification changes.
// The checksum still comes from the project's own HTTPS site; the peer is
// asked only whether it holds a file with exactly that hash, and the bytes it
// sends are checked against that same checksum before anything is placed. A
// peer that is stale, broken, or not really an isoshelf costs one fall back
// to the real source. So this package never has to be trusted - only reached.
package peer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// A Client is another isoshelf, signed in to.
type Client struct {
	// Address is host:port, without a scheme.
	Address string
	// HTTP carries the session cookie once signed in. Made by New.
	HTTP *http.Client
	// FolderID and FolderName say which folder is asking, so the isoshelf
	// doing the sharing can keep track of the drives it has served - and, one
	// day, rebuild one that is lost. Neither is a credential; the session
	// cookie is what decides whether anything is handed over.
	FolderID   string
	FolderName string
	user       string
}

// Headers returns what identifies this folder, for the fetcher to send on
// the download itself.
func (c *Client) Headers() map[string]string {
	if c.FolderID == "" {
		return nil
	}
	return map[string]string{
		"X-Isoshelf-Folder":      c.FolderID,
		"X-Isoshelf-Folder-Name": c.FolderName,
	}
}

// New returns a client for the isoshelf at address. Nothing is sent until
// SignIn or a question is asked.
func New(address string) (*Client, error) {
	address = strings.TrimSpace(address)
	address = strings.TrimPrefix(strings.TrimPrefix(address, "http://"), "https://")
	address = strings.TrimSuffix(address, "/")
	if address == "" {
		return nil, errors.New("Give the address of the other isoshelf, like 10.0.0.5:8765.")
	}
	if strings.ContainsAny(address, "/?#") {
		return nil, errors.New("That looks like a whole link. Just the address and port, like 10.0.0.5:8765.")
	}
	if !strings.Contains(address, ":") {
		address += ":8765" // the port isoshelf uses unless told otherwise
	}
	// A jar rather than a header, because the session cookie is what the
	// other isoshelf hands out and what it expects back - including on the
	// download itself, which goes through the ordinary fetcher.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{Address: address, HTTP: &http.Client{Jar: jar, Timeout: 30 * time.Second}}, nil
}

// SignIn signs in and keeps the session. It is worth calling once when the
// address is saved, so the page can say whether it worked while somebody is
// still looking at the box they typed it into.
func (c *Client) SignIn(ctx context.Context, user, password string) error {
	form := url.Values{"user": {user}, "password": {password}}
	// The form's own value first: the other isoshelf refuses a login form it
	// didn't draw, and sets a cookie with the value it expects back.
	page, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/login", nil)
	if err != nil {
		return err
	}
	res, err := c.HTTP.Do(page)
	if err != nil {
		return c.cantReach(err)
	}
	res.Body.Close()
	for _, cookie := range c.HTTP.Jar.Cookies(mustParse(c.base() + "/login")) {
		if cookie.Name == "isoshelf_form" {
			form.Set("form_token", cookie.Value)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Don't follow the redirect a good login answers with: the cookie is
	// already in the jar by then, and the page itself is of no interest.
	noRedirect := *c.HTTP
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err = noRedirect.Do(req)
	if err != nil {
		return c.cantReach(err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusSeeOther || res.StatusCode == http.StatusOK && c.signedIn():
		c.user = user
		return nil
	case res.StatusCode == http.StatusUnauthorized:
		return errors.New("That username and password didn't work on the other isoshelf.")
	case res.StatusCode == http.StatusTooManyRequests:
		return errors.New("The other isoshelf is refusing sign-ins for a moment after too many wrong tries. Wait and try again.")
	}
	return fmt.Errorf("The other isoshelf answered %s when signing in.", res.Status)
}

// signedIn says whether the jar holds a session from the other isoshelf.
func (c *Client) signedIn() bool {
	for _, cookie := range c.HTTP.Jar.Cookies(mustParse(c.base())) {
		if cookie.Name == "isoshelf_session" && cookie.Value != "" {
			return true
		}
	}
	return false
}

// Have asks whether the peer holds this exact file: the name and the hash
// both have to match, so a stale copy under the same name is a no.
func (c *Client) Have(ctx context.Context, filename, sha256 string) (bool, int64, error) {
	ask := c.base() + "/api/share/have?" + url.Values{"name": {filename}, "sha256": {sha256}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ask, nil)
	if err != nil {
		return false, 0, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return false, 0, c.cantReach(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false, 0, fmt.Errorf("the other isoshelf answered %s", res.Status)
	}
	var answer struct {
		Have bool  `json:"have"`
		Size int64 `json:"size"`
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		return false, 0, fmt.Errorf("the other isoshelf gave an answer isoshelf couldn't read: %w", err)
	}
	return answer.Have, answer.Size, nil
}

// FileURL is where to fetch the file from, for the ordinary fetcher to use
// like any other place a file can come from.
func (c *Client) FileURL(filename, sha256 string) string {
	return c.base() + "/api/share/file/" + url.PathEscape(filename) + "?sha256=" + url.QueryEscape(sha256)
}

// base is the peer's address with a scheme. Plain http: this is a machine on
// the same network, reached by address, with no certificate anybody could
// check. The password travels over it, which the page says plainly.
func (c *Client) base() string { return "http://" + c.Address }

func (c *Client) cantReach(err error) error {
	return fmt.Errorf("Couldn't reach an isoshelf at %s: %w", c.Address, err)
}

func mustParse(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}
	return u
}
