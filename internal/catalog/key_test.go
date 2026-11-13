package catalog

import (
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/verify/verifytest"
)

// fakeFingerprint is the fingerprint the test catalogs are written with;
// load swaps it for the generated key's own, so the text stays readable.
const fakeFingerprint = "0123 4567 89AB CDEF 0123  4567 89AB CDEF 0123 4567"

// testKey is a real signing key, made once for these tests.
func testKey() (string, string) {
	k := verifytest.NewKey("catalog test")
	return k.Armored, k.Fingerprint
}

// withTestKey puts the generated key's fingerprint where the fake one is,
// written the way people copy one - in groups, in lower case - so loading
// still has to tidy it.
func withTestKey(text string) string {
	_, fp := testKey()
	var groups []string
	for i := 0; i < len(fp); i += 4 {
		groups = append(groups, strings.ToLower(fp[i:i+4]))
	}
	return strings.ReplaceAll(text, fakeFingerprint, strings.Join(groups, " "))
}

// A key that doesn't hold the fingerprint the catalog names is refused when
// the catalog loads, and an armored key written in the catalog works the
// same as a file.
func TestSigningKeysAreCheckedAgainstTheirFingerprints(t *testing.T) {
	if _, err := load(t, strings.Replace(validCatalog, fakeFingerprint, strings.Repeat("AB", 20), 1)); err == nil ||
		!strings.Contains(err.Error(), "doesn't hold the key") {
		t.Errorf("a key with the wrong fingerprint was accepted: %v", err)
	}
	armored, _ := testKey()
	inline := strings.Replace(validCatalog, `file = "keys/example.asc"`, "armored = '''\n"+armored+"'''", 1)
	c, err := load(t, inline)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range c.Entries {
		if e.Artifact != nil && e.Artifact.Signature != "" && e.Artifact.Signer() == nil {
			t.Errorf("%s is signed but has no key", e.ID)
		}
	}
	both := strings.Replace(inline, "armored = '''", "file = \"keys/example.asc\"\narmored = '''", 1)
	if _, err := load(t, both); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("a key with a file and an armored copy was accepted: %v", err)
	}
}
