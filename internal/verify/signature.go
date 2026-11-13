package verify

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
)

// OpenPGP signatures (#5). A checksum proves the bytes match what the
// checksum file says; it proves nothing if the checksum file itself was
// replaced. Where a project signs its checksum file (or the image), isoshelf
// checks the signature against the project's key, and a signature that
// fails stops the download exactly as a checksum mismatch does: nothing is
// placed, nothing is replaced.
//
// The key is never fetched alongside what it checks. It sits in the
// catalog, which this repository publishes, with the fingerprints the
// project's own documentation gives; a key that doesn't carry every one of
// them is refused when the catalog loads.

// ErrBadSignature means a signature didn't verify: it was made by another
// key, or the file isn't what was signed.
var ErrBadSignature = errors.New("the signature doesn't match")

// Keyring is a project's signing keys, already checked against their
// fingerprints.
type Keyring struct {
	list openpgp.EntityList
}

// ReadKeyring reads an armored key file and keeps the keys with the given
// primary fingerprints (uppercase hex, no spaces). Every fingerprint must be
// there.
func ReadKeyring(armored []byte, fingerprints []string) (*Keyring, error) {
	all, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(armored))
	if err != nil {
		return nil, fmt.Errorf("the signing key can't be read: %w", err)
	}
	var kept openpgp.EntityList
	for _, want := range fingerprints {
		found := false
		for _, e := range all {
			if strings.EqualFold(fmt.Sprintf("%X", e.PrimaryKey.Fingerprint), want) {
				kept, found = append(kept, e), true
			}
		}
		if !found {
			return nil, fmt.Errorf("the signing key file doesn't hold the key %s", want)
		}
	}
	return &Keyring{list: kept}, nil
}

// CheckDetached checks a detached signature, armored or binary, over signed.
func (k *Keyring) CheckDetached(signed io.Reader, signature []byte) error {
	var err error
	if bytes.HasPrefix(bytes.TrimSpace(signature), []byte("-----BEGIN PGP SIGNATURE-----")) {
		_, err = openpgp.CheckArmoredDetachedSignature(k.list, signed, bytes.NewReader(signature), nil)
	} else {
		_, err = openpgp.CheckDetachedSignature(k.list, signed, bytes.NewReader(signature), nil)
	}
	if err != nil {
		return fmt.Errorf("%w (%v)", ErrBadSignature, err)
	}
	return nil
}

// Clearsigned checks a clearsigned file and returns only the text the
// signature covers: anything before or after it in the file is dropped, so
// nothing unsigned can be read as a checksum.
func (k *Keyring) Clearsigned(data []byte) ([]byte, error) {
	block, _ := clearsign.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%w (the file isn't clearsigned)", ErrBadSignature)
	}
	if _, err := block.VerifySignature(k.list, nil); err != nil {
		return nil, fmt.Errorf("%w (%v)", ErrBadSignature, err)
	}
	return block.Plaintext, nil
}
