// Package auth keeps the username and password that let someone into
// isoshelf when it runs on a network rather than on one person's computer.
//
// The secret in the link is still there and still works. This is the thing
// people expect instead: a login box, one username and one password, set once
// and used from every device. What it is NOT is a user system - isoshelf
// looks after a folder of files, and there is exactly one person's worth of
// access to give.
//
// The password is never stored, only a hash of it: PBKDF2-HMAC-SHA256 with a
// random salt, from the standard library, so isoshelf keeps its one
// dependency. Nobody who reads the file can work the password back out of it.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// FileName is the account file inside isoshelf's own folder.
const FileName = "login.json"

const (
	// iterations is how much work checking one password takes. The number
	// follows OWASP's advice for PBKDF2-HMAC-SHA256, and is written into the
	// file so it can be raised later without locking anyone out.
	iterations = 600_000
	keyLength  = 32
	saltLength = 16

	// MinPassword is short enough that nobody gives up and long enough to be
	// worth typing. This is a home network, not a bank, and a rule people
	// work around by choosing "Password1" has made things worse.
	MinPassword = 8
	maxPassword = 1024 // a whole paste of a password manager's output, no more
)

// Account is what the file holds. The password is not in it.
type Account struct {
	User string `json:"user"`
	// Salt and Hash are base64. Iterations is kept so a file written by an
	// older isoshelf still checks correctly.
	Salt       string    `json:"salt"`
	Hash       string    `json:"hash"`
	Iterations int       `json:"iterations"`
	SetAt      time.Time `json:"set_at"`
}

// Load reads the account, or returns nil when nobody has set one.
func Load(dir string) (*Account, error) {
	if dir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var a Account
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	if a.User == "" || a.Salt == "" || a.Hash == "" || a.Iterations <= 0 {
		return nil, fmt.Errorf("%s: not a usable login", FileName)
	}
	return &a, nil
}

// Set writes the account, replacing whatever was there. It is the only way a
// password gets in, so the rules about what a password may be live here.
func Set(dir, user, password string) error {
	user = strings.TrimSpace(user)
	if err := CheckNew(user, password); err != nil {
		return err
	}
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLength)
	if err != nil {
		return err
	}
	a := Account{
		User:       user,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Hash:       base64.StdEncoding.EncodeToString(key),
		Iterations: iterations,
		SetAt:      time.Now().UTC(),
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// 0600: it is not the password, but it is what a password is checked
	// against, and nobody else on the machine needs to read it.
	return writeSecret(filepath.Join(dir, FileName), append(data, '\n'))
}

// CheckNew says whether a username and password may be used, in the words the
// page shows.
func CheckNew(user, password string) error {
	switch {
	case strings.TrimSpace(user) == "":
		return errors.New("Choose a username.")
	case utf8.RuneCountInString(user) > 64:
		return errors.New("That username is too long.")
	case strings.ContainsAny(user, "\n\r\x00"):
		return errors.New("A username can't have line breaks in it.")
	case utf8.RuneCountInString(password) < MinPassword:
		return fmt.Errorf("The password needs at least %d characters.", MinPassword)
	case len(password) > maxPassword:
		return errors.New("That password is too long.")
	}
	return nil
}

// Matches says whether this username and password are the account's. It takes
// the same time whether the username is wrong, the password is wrong or both,
// so that trying names against it tells nobody anything.
func (a *Account) Matches(user, password string) bool {
	salt, err := base64.StdEncoding.DecodeString(a.Salt)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(a.Hash)
	if err != nil {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, a.Iterations, len(want))
	if err != nil {
		return false
	}
	sameUser := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(user)), []byte(a.User))
	samePassword := subtle.ConstantTimeCompare(key, want)
	return sameUser&samePassword == 1
}

// writeSecret writes a file only its owner can read, through a temporary file
// so a crash never leaves half of one.
func writeSecret(name string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, fs.ErrInvalid) {
		// Windows has no mode bits to set; that is not a reason to stop.
		_ = err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), name)
}
