// Package verify checks images against published checksums and signatures.
package verify

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Algorithm is a checksum algorithm.
type Algorithm string

const (
	MD5    Algorithm = "md5"
	SHA1   Algorithm = "sha1"
	SHA256 Algorithm = "sha256"
	SHA512 Algorithm = "sha512"
)

// Weak reports whether the algorithm only guards against accidental
// corruption, not deliberate tampering.
func (a Algorithm) Weak() bool {
	return a == MD5 || a == SHA1
}

func (a Algorithm) strength() int {
	switch a {
	case MD5:
		return 1
	case SHA1:
		return 2
	case SHA256:
		return 3
	case SHA512:
		return 4
	}
	return 0
}

var hexLength = map[int]Algorithm{32: MD5, 40: SHA1, 64: SHA256, 128: SHA512}

// Checksum is one line of a manifest.
type Checksum struct {
	// Name is the file's base name.
	Name      string
	Algorithm Algorithm
	// Hex is the digest in lowercase hex.
	Hex string
}

var (
	gnuLine = regexp.MustCompile(`^([0-9a-fA-F]+)\s+\*?(.+)$`)
	bsdLine = regexp.MustCompile(`^(MD5|SHA-?1|SHA-?256|SHA-?512)\s*\((.+)\)\s*=\s*([0-9a-fA-F]+)$`)
)

// ParseManifest reads checksum lines in GNU form ("<hash>  <file>" or
// "<hash> *<file>") and BSD form ("SHA256 (<file>) = <hash>"). Blank lines,
// comments, OpenPGP clearsign armor and lines it doesn't understand are
// skipped. Signatures are not checked here.
func ParseManifest(data []byte) []Checksum {
	var out []Checksum
	inSignature, inArmorHeader := false, false
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "-----BEGIN PGP SIGNED MESSAGE-----":
			inArmorHeader = true
			continue
		case inArmorHeader:
			inArmorHeader = line != "" // "Hash: SHA256" lines end at a blank line
			continue
		case line == "-----BEGIN PGP SIGNATURE-----":
			inSignature = true
			continue
		case line == "-----END PGP SIGNATURE-----":
			inSignature = false
			continue
		case inSignature || line == "" || strings.HasPrefix(line, "#"):
			continue
		}
		line = strings.TrimPrefix(line, "- ") // dash-escaped clearsigned text

		if m := bsdLine.FindStringSubmatch(line); m != nil {
			algo := Algorithm(strings.ToLower(strings.ReplaceAll(m[1], "-", "")))
			if hexLength[len(m[3])] == algo {
				out = append(out, Checksum{Name: baseName(m[2]), Algorithm: algo, Hex: strings.ToLower(m[3])})
			}
			continue
		}
		if m := gnuLine.FindStringSubmatch(line); m != nil {
			if algo, ok := hexLength[len(m[1])]; ok {
				out = append(out, Checksum{Name: baseName(m[2]), Algorithm: algo, Hex: strings.ToLower(m[1])})
			}
		}
	}
	return out
}

// Strongest returns the strongest checksum listed for name.
func Strongest(checksums []Checksum, name string) (Checksum, bool) {
	var best Checksum
	found := false
	for _, c := range checksums {
		if c.Name == name && (!found || c.Algorithm.strength() > best.Algorithm.strength()) {
			best, found = c, true
		}
	}
	return best, found
}

func baseName(name string) string {
	return path.Base(strings.ReplaceAll(strings.TrimSpace(name), `\`, "/"))
}

// Hash computes a file's digest with the given algorithm, as lowercase hex.
// It stops when ctx is cancelled, and reports the bytes read so far to
// progress, if not nil.
func Hash(ctx context.Context, name string, algo Algorithm, progress func(done int64)) (string, error) {
	h, err := New(algo)
	if err != nil {
		return "", err
	}
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 1<<20)
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		h.Write(buf[:n])
		done += int64(n)
		if progress != nil && n > 0 {
			progress(done)
		}
		if err == io.EOF {
			return hex.EncodeToString(h.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}

// New returns a new hash for the algorithm.
func New(algo Algorithm) (hash.Hash, error) {
	switch algo {
	case MD5:
		return md5.New(), nil
	case SHA1:
		return sha1.New(), nil
	case SHA256:
		return sha256.New(), nil
	case SHA512:
		return sha512.New(), nil
	}
	return nil, fmt.Errorf("unknown checksum algorithm %q", algo)
}

// Mismatch is returned when a file doesn't match its published checksum.
type Mismatch struct {
	Name     string
	Expected Checksum
	Got      string
}

func (e *Mismatch) Error() string {
	return fmt.Sprintf("%s doesn't match its published %s checksum (expected %s, got %s)",
		e.Name, e.Expected.Algorithm, e.Expected.Hex, e.Got)
}

// Check reads a file and compares it with a published checksum. A mismatch
// returns a *Mismatch error.
func Check(ctx context.Context, name string, c Checksum, progress func(done int64)) error {
	got, err := Hash(ctx, name, c.Algorithm, progress)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, c.Hex) {
		return &Mismatch{Name: filepath.Base(name), Expected: c, Got: got}
	}
	return nil
}

// MustNew returns a new hash for an algorithm the caller knows is supported.
func MustNew(algo Algorithm) hash.Hash {
	h, err := New(algo)
	if err != nil {
		panic(err)
	}
	return h
}
