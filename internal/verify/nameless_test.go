package verify

import "testing"

// A checksum file for one image that doesn't name it (v0.8.10): AnduinOS
// writes "SHA256: <hash>", others the hash alone. Either belongs to the
// image the file was fetched for; a label that doesn't fit the hash's
// length is not believed.
func TestAChecksumWithoutAName(t *testing.T) {
	const h = "d838db7762cf21abb1df983b7718c008fbc10c60bb68845d48dbd266b358edde"
	for _, text := range []string{"SHA256: " + h + "\n", h + "\n"} {
		got := ParseManifest([]byte(text))
		c, ok := Strongest(got, "AnduinOS-2.0.4-amd64.iso")
		if !ok || c.Algorithm != SHA256 || c.Hex != h {
			t.Errorf("%q read as %+v", text, got)
		}
	}
	if got := ParseManifest([]byte("MD5: " + h + "\n")); len(got) != 0 {
		t.Errorf("an MD5 label on a SHA-256-length hash read as %+v", got)
	}
}
