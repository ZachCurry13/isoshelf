package web

import (
	"encoding/json"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/auth"
)

// Changing the login, from inside the page.
//
// These run behind the guard, so whoever is asking is already in. What still
// has to be checked is that they know the current password - a browser left
// open on somebody's desk should not be enough to change it.
//
// Somebody who has forgotten it doesn't come through here at all: the way
// back is ISOSHELF_USERNAME and ISOSHELF_PASSWORD at the next start, or
// "isoshelf password" on the machine itself. Both need the machine, which is
// the right bar, and neither is a second door standing open on the network.

// loginJSON is what Settings shows about the login.
type loginJSON struct {
	// User is who is set, or "" when nobody is.
	User string `json:"user,omitempty"`
	// CanSet says whether this isoshelf offers a login at all: a desktop
	// that only answers to itself has nothing to protect with one.
	CanSet bool `json:"can_set"`
}

func (s *Server) loginInfo() loginJSON {
	out := loginJSON{CanSet: s.cfg.AnyHost}
	if a := s.account(); a != nil {
		out.User, out.CanSet = a.User, true
	}
	return out
}

// setLogin sets or changes the username and password.
func (s *Server) setLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User     string `json:"user"`
		Current  string `json:"current"`
		Password string `json:"password"`
		Again    string `json:"again"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	have := s.account()
	// Changing it means proving you still know it. A browser somebody left
	// open should not be enough.
	if have != nil && !have.Matches(have.User, req.Current) {
		writeError(w, http.StatusForbidden, "That isn't the current password.")
		return
	}
	if req.Password != req.Again {
		writeError(w, http.StatusBadRequest, "Those two passwords aren't the same.")
		return
	}
	user := req.User
	if user == "" && have != nil {
		user = have.User // changing only the password
	}
	if err := auth.CheckNew(user, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.Set(s.cfg.Dirs.Config, user, req.Password); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the login: "+err.Error())
		return
	}
	// This browser keeps its place; every other one is asked again, because
	// a changed password should mean a changed password everywhere.
	s.rotateSessionKey()
	s.setSession(w, user)
	s.getState(w, r)
}

// signOutEverywhere throws the signing key away, which stops every session
// isoshelf has handed out, this browser's included.
func (s *Server) signOutEverywhere(w http.ResponseWriter, r *http.Request) {
	if s.account() == nil {
		writeError(w, http.StatusBadRequest, "There's no login to sign out of yet.")
		return
	}
	s.rotateSessionKey()
	s.setSession(w, "")
	writeJSON(w, http.StatusOK, map[string]any{"status": "signed out"})
}

// rotateSessionKey makes a new signing key, so every cookie signed with the
// old one stops being worth anything.
func (s *Server) rotateSessionKey() {
	auth.Forget(s.cfg.Dirs.Config)
	if key, saved, err := auth.LoadKey(s.cfg.Dirs.Config); err == nil {
		s.mu.Lock()
		s.sessions, s.keyIsSaved = key, saved
		s.mu.Unlock()
	}
}
