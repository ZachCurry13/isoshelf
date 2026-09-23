package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/peer"
	"github.com/ZachCurry13/isoshelf/internal/settings"
)

// Using another isoshelf on the network as the first place to look.
//
// See internal/peer for why this is safe to prefer: the checksum still comes
// from the project's own site, and a copy that doesn't match it costs one
// fall back rather than a bad file.

// nearerTimeout is how long to wait for the peer to say whether it has a
// file. It is a machine on the same network; if it doesn't answer in a few
// seconds it is asleep or gone, and waiting longer only delays a download
// that was going to happen anyway.
const nearerTimeout = 5 * time.Second

// peerFor signs in to the other isoshelf, if one is set up, and returns a
// client for this download. Nil means there is nobody to ask, or nobody
// answering - either way the download goes the long way round.
//
// Built per download rather than kept: a peer edited on the page takes
// effect on the next one, and nothing about one download is left lying
// around for the next to trip over.
func (s *Server) peerFor(target string) *peer.Client {
	saved := s.loadSettings().Peer
	if !saved.Use() {
		return nil
	}
	client, err := peer.New(saved.Address)
	if err != nil {
		return nil
	}
	client.FolderName = filepath.Base(target)
	s.mu.Lock()
	if s.st != nil && s.target == target {
		client.FolderID = s.st.TargetID
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), nearerTimeout)
	defer cancel()
	if saved.User != "" {
		if err := client.SignIn(ctx, saved.User, saved.Password); err != nil {
			s.noteAboutPeer("Couldn't sign in to " + saved.Address + ", so downloads are coming from the internet.")
			return nil
		}
	}
	return client
}

// nearer is the question update.Run asks before downloading: has the peer
// got this exact file?
func nearer(client *peer.Client, note func(string)) func(filename, sha256 string) []string {
	if client == nil {
		return nil
	}
	return func(filename, sha256 string) []string {
		ctx, cancel := context.WithTimeout(context.Background(), nearerTimeout)
		defer cancel()
		have, _, err := client.Have(ctx, filename, sha256)
		if err != nil {
			note("Couldn't reach " + client.Address + ", so this came from the internet.")
			return nil
		}
		if !have {
			return nil
		}
		return []string{client.FileURL(filename, sha256)}
	}
}

// noteAboutPeer puts one line on the page. A peer that can't be reached is
// not a failure - the download still happens, just the long way round - but
// silently doing the slow thing is how somebody ends up wondering why their
// NAS made no difference.
func (s *Server) noteAboutPeer(note string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, already := range s.warnings {
		if already == note {
			return
		}
	}
	s.warnings = append(s.warnings, note)
}

// peerJSON is what Settings shows about the other isoshelf.
type peerJSON struct {
	Address string `json:"address,omitempty"`
	User    string `json:"user,omitempty"`
	// On is whether it is being used. Off keeps the address without asking.
	On bool `json:"on"`
	// Sharing is whether this isoshelf offers its own images to others.
	Sharing bool `json:"sharing"`
}

func (s *Server) peerInfo(saved settings.Settings) peerJSON {
	return peerJSON{
		Address: saved.Peer.Address,
		User:    saved.Peer.User,
		On:      saved.Peer.Use(),
		Sharing: saved.ShareImages != nil && *saved.ShareImages,
	}
}

// setPeer saves the other isoshelf's address and signs in to check it, so
// that a typo is found while somebody is still looking at the box.
func (s *Server) setPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address  string `json:"address"`
		User     string `json:"user"`
		Password string `json:"password"`
		// Off turns it off without forgetting the address; Forget clears it.
		Off    *bool `json:"off"`
		Forget bool  `json:"forget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.Forget {
		s.updateSettings(func(c *settings.Settings) { c.Peer = settings.Peer{} })
		s.getState(w, r)
		return
	}
	if req.Off != nil && req.Address == "" {
		s.updateSettings(func(c *settings.Settings) { c.Peer.Off = *req.Off })
		s.getState(w, r)
		return
	}

	client, err := peer.New(req.Address)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Reaching it now is the whole point of asking: an address that only
	// fails at the next download fails somewhere nobody is watching.
	ctx, cancel := context.WithTimeout(r.Context(), nearerTimeout)
	defer cancel()
	password := req.Password
	if password == "" {
		password = s.loadSettings().Peer.Password // unchanged, so reuse it
	}
	if req.User != "" {
		if err := client.SignIn(ctx, req.User, password); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if _, _, err := client.Have(ctx, "isoshelf-checking-this-address", "0"); err != nil {
		writeError(w, http.StatusBadRequest,
			"Reached something at "+client.Address+", but it didn't answer like an isoshelf that's sharing its images. Check the address, and that sharing is turned on over there.")
		return
	}
	s.updateSettings(func(c *settings.Settings) {
		c.Peer = settings.Peer{Address: client.Address, User: req.User, Password: password}
	})
	s.getState(w, r)
}

// withJar returns a client like base but carrying jar, without touching base:
// it is shared, and a cookie jar bolted onto it would outlive this download.
func withJar(base *http.Client, jar http.CookieJar) *http.Client {
	out := &http.Client{Jar: jar}
	if base != nil {
		out.Transport, out.CheckRedirect, out.Timeout = base.Transport, base.CheckRedirect, base.Timeout
	}
	return out
}

// sourceName says where a download's bytes are arriving from, in words a
// person can act on: the name of the other isoshelf, or "the internet".
//
// Copying from another isoshelf (v0.4.9) is the one feature whose whole value
// is that it is faster, and it was invisible while it happened - the dock read
// "Downloading 40%" whether the bytes were crossing the room or an ocean. It
// also means somebody can see that their peer setting is doing nothing, which
// short of watching a router was not findable at all.
//
// It names isoshelf rather than the machine, at the maintainer's asking: the
// folder's own name ("the NAS") said where the bytes were without saying what
// was serving them, and the thing worth knowing is that the other isoshelf
// answered at all. There is only ever one peer set up, so nothing is
// ambiguous for want of the address.
func sourceName(rawURL string, near *peer.Client) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	if near != nil && sameHost(u, near.Address) {
		return "your isoshelf server"
	}
	return "the internet"
}

// sameHost reports whether a URL points at the address a peer was set up
// with. The peer's address may or may not carry a scheme, so it is compared
// as a host and port rather than as text.
func sameHost(u *url.URL, address string) bool {
	if address == "" {
		return false
	}
	if !strings.Contains(address, "//") {
		address = "http://" + address
	}
	other, err := url.Parse(address)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, other.Host)
}
