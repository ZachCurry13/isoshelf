// Package verifytest makes OpenPGP keys and signatures for tests, so nothing
// about signature checking is tested against a key that isn't real.
package verifytest

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
)

// Key is a signing key made for a test.
type Key struct {
	entity *openpgp.Entity
	// Armored is the public key, as a project publishes it.
	Armored string
	// Fingerprint is its primary fingerprint in uppercase hex.
	Fingerprint string
}

var made = map[string]*Key{}
var mu sync.Mutex

// NewKey makes a key, once per name: making one takes a moment.
func NewKey(name string) *Key {
	mu.Lock()
	defer mu.Unlock()
	if k := made[name]; k != nil {
		return k
	}
	e, err := openpgp.NewEntity(name, "", name+"@example.org", nil)
	if err != nil {
		panic(err)
	}
	var b bytes.Buffer
	w, err := armor.Encode(&b, openpgp.PublicKeyType, nil)
	if err != nil {
		panic(err)
	}
	if err := e.Serialize(w); err != nil {
		panic(err)
	}
	w.Close()
	k := &Key{entity: e, Armored: b.String(), Fingerprint: fmt.Sprintf("%X", e.PrimaryKey.Fingerprint)}
	made[name] = k
	return k
}

// Detached signs data, armored when asked, binary otherwise.
func (k *Key) Detached(data []byte, armored bool) []byte {
	var b bytes.Buffer
	var err error
	if armored {
		err = openpgp.ArmoredDetachSign(&b, k.entity, bytes.NewReader(data), nil)
	} else {
		err = openpgp.DetachSign(&b, k.entity, bytes.NewReader(data), nil)
	}
	if err != nil {
		panic(err)
	}
	return b.Bytes()
}

// Clearsign wraps data in a clearsigned message.
func (k *Key) Clearsign(data []byte) []byte {
	var b bytes.Buffer
	w, err := clearsign.Encode(&b, k.entity.PrivateKey, nil)
	if err != nil {
		panic(err)
	}
	w.Write(data)
	w.Close()
	return b.Bytes()
}
