// Package verify checks images against published checksums and signatures.
package verify

import (
	"bufio"
	"bytes"
	"path"
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
