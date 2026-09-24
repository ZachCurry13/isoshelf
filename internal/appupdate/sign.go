package appupdate

import (
	"crypto/ed25519"
	_ "embed"
	"encoding/base64"
	"errors"
	"strings"
)

// A release is signed so that isoshelf installs only what this project
// published (decision 14). The checksum file is what gets signed: it names
// every file in the release with its SHA-256, and the names carry the version,
// so one signature vouches for every byte and for which release they belong
// to. A checksum on its own proves nothing if the checksum file itself was
// replaced; the signature is what closes that.
//
// The key is Ed25519, from the standard library, so this costs no dependency.
// The private half lives only in the repository's secrets, where the release
// workflow reads it; the public half is this file, built into every isoshelf.

// SignatureSuffix is added to the checksum file's name for its signature.
const SignatureSuffix = ".sig"

// SumsFile is the checksum file every release carries.
const SumsFile = "SHA256SUMS"

//go:embed release.pub
var releasePub string

// releaseKey is the key releases are checked against. ok is false for a build
// made before the key was set up, which then cannot update itself: it has
// nothing to check an update against, and an update it can't check is one it
// must not install.
func releaseKey() (key ed25519.PublicKey, ok bool) {
	return ParsePublicKey(releasePub)
}

// HasKey says whether this build can check an update at all.
func HasKey() bool {
	_, ok := releaseKey()
	return ok
}

// ParsePublicKey reads a public key written by EncodeKey.
func ParsePublicKey(text string) (ed25519.PublicKey, bool) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, false
	}
	return ed25519.PublicKey(raw), true
}

// ParsePrivateKey reads a private key written by EncodeKey.
func ParsePrivateKey(text string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return nil, errors.New("the signing key isn't valid base64")
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	}
	return nil, errors.New("the signing key is the wrong length")
}

// EncodeKey writes a key as one line of base64.
func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// Sign returns the signature of data, as the text a .sig file holds.
func Sign(key ed25519.PrivateKey, data []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, data)) + "\n"
}

// ErrBadSignature means a file doesn't carry this project's signature.
var ErrBadSignature = errors.New("the update isn't signed by the isoshelf project, so it wasn't installed")

// VerifyRelease checks sig against the key built into isoshelf. The release
// workflow uses it on its own signature, so a release signed with a key that
// doesn't match fails there rather than on everybody's computer.
func VerifyRelease(data []byte, sig string) error {
	key, ok := releaseKey()
	if !ok {
		return ErrNoKey
	}
	return Verify(key, data, sig)
}

// Verify checks that sig is key's signature of data.
func Verify(key ed25519.PublicKey, data []byte, sig string) error {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sig))
	if err != nil || !ed25519.Verify(key, data, raw) {
		return ErrBadSignature
	}
	return nil
}
