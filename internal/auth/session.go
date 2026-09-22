package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// A session is what a browser holds after someone has logged in, so the
// password is typed once rather than on every page.
//
// It is signed rather than remembered: the cookie says who and until when,
// with a signature isoshelf can check but nobody else can make. That way a
// restart doesn't log everyone out - which matters, because isoshelf on a NAS
// restarts whenever the app updates - and isoshelf keeps no list of who is
// logged in. Signing out everywhere is throwing the key away and making a
// new one.

// KeyFileName holds the signing key, inside isoshelf's own folder.
const KeyFileName = "session-key"

// SessionLife is how long a login lasts. Long, because this is a tool on a
// home network and being asked to log in again every day is the kind of thing
// that makes people turn the whole idea off.
const SessionLife = 30 * 24 * time.Hour

// Key signs sessions. It is made on first use and kept in isoshelf's folder.
type Key []byte

// LoadKey reads the signing key, making one if there isn't one yet. A folder
// it can't write to isn't fatal: a key held only in memory still works, it
// just means a restart asks for the password again, and saying so is better
// than refusing to start.
func LoadKey(dir string) (Key, error) {
	if dir == "" {
		return newKey()
	}
	path := filepath.Join(dir, KeyFileName)
	if raw, err := os.ReadFile(path); err == nil {
		if key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw))); err == nil && len(key) >= 32 {
			return key, nil
		}
		// Unreadable or too short to be a key: replaced rather than used.
	} else if !errors.Is(err, fs.ErrNotExist) {
		return newKey() // can't read it; carry on with one in memory
	}
	key, err := newKey()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		writeSecret(path, []byte(base64.StdEncoding.EncodeToString(key)+"\n"))
	}
	return key, nil
}

// Forget throws the signing key away, so every session signed with it stops
// working. That is what "sign out everywhere" means here.
func Forget(dir string) error {
	if dir == "" {
		return nil
	}
	err := os.Remove(filepath.Join(dir, KeyFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func newKey() (Key, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// Mint returns the cookie value for someone who has just logged in.
func (k Key) Mint(user string, now time.Time) string {
	body := fmt.Sprintf("%s|%d", base64.RawURLEncoding.EncodeToString([]byte(user)), now.Add(SessionLife).Unix())
	return body + "|" + k.sign(body)
}

// Valid says whether a cookie is one isoshelf signed and hasn't expired.
func (k Key) Valid(cookie string, now time.Time) bool {
	// The signature is everything after the last separator; the body is the
	// rest, separators and all.
	cut := strings.LastIndex(cookie, "|")
	if cut < 0 {
		return false
	}
	body, sig := cookie[:cut], cookie[cut+1:]
	if subtle.ConstantTimeCompare([]byte(sig), []byte(k.sign(body))) != 1 {
		return false
	}
	_, until, ok := strings.Cut(body, "|")
	if !ok {
		return false
	}
	seconds, err := strconv.ParseInt(until, 10, 64)
	return err == nil && now.Before(time.Unix(seconds, 0))
}

func (k Key) sign(body string) string {
	mac := hmac.New(sha256.New, k)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
