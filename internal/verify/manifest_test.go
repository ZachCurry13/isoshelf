package verify

import (
	"slices"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

const (
	sha256a = "a081ab202cfda17f6924128dbd2de8b63518ac0531bcfe3f1a1b88097c459bd4"
	sha256b = "bcbde5d345c5013fa618c38380765547be01a354883b3055f32d7067dd7b5bca"
	md5a    = "0123456789abcdef0123456789abcdef"
)

func TestParseManifest(t *testing.T) {
	manifest := strings.Join([]string{
		"-----BEGIN PGP SIGNED MESSAGE-----",
		"Hash: SHA256",
		"",
		"# Fedora-Workstation-Live-44.iso: 2400000000 bytes",
		sha256a + " *linuxmint-22.3-cinnamon-64bit.iso",
		strings.ToUpper(sha256b) + "  ./isos/CentOS-7-i386-Minimal-2009.iso\r",
		"SHA256 (Fedora-Workstation-Live-44.iso) = " + sha256a,
		"MD5 (Core-16.2.iso) = " + md5a,
		md5a + "  Core-16.2.iso",
		"SHA256 (short.iso) = abcd",
		"not a checksum line",
		"-----BEGIN PGP SIGNATURE-----",
		"iQIzBAEBCAAdFiEE" + sha256a,
		"-----END PGP SIGNATURE-----",
	}, "\n")

	got := ParseManifest([]byte(manifest))
	want := []Checksum{
		{"linuxmint-22.3-cinnamon-64bit.iso", SHA256, sha256a},
		{"CentOS-7-i386-Minimal-2009.iso", SHA256, sha256b},
		{"Fedora-Workstation-Live-44.iso", SHA256, sha256a},
		{"Core-16.2.iso", MD5, md5a},
		{"Core-16.2.iso", MD5, md5a},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%v\nwant:\n%v", got, want)
	}
}

func TestStrongest(t *testing.T) {
	checksums := []Checksum{
		{"a.iso", MD5, md5a},
		{"a.iso", SHA256, sha256a},
		{"a.iso", SHA1, "0123456789abcdef0123456789abcdef01234567"},
		{"b.iso", MD5, md5a},
	}
	if c, ok := Strongest(checksums, "a.iso"); !ok || c.Algorithm != SHA256 {
		t.Errorf("a.iso: got %v, %v; want sha256", c, ok)
	}
	if c, ok := Strongest(checksums, "b.iso"); !ok || !c.Algorithm.Weak() {
		t.Errorf("b.iso: got %v, %v; want a weak md5", c, ok)
	}
	if _, ok := Strongest(checksums, "c.iso"); ok {
		t.Error("c.iso: found a checksum that isn't there")
	}
}

// The recorded manifests from real projects all parse.
func TestParseRecordedManifests(t *testing.T) {
	tests := map[string]string{
		"https://mirrors.edge.kernel.org/linuxmint/stable/22.3/sha256sum.txt":                   "linuxmint-22.3-cinnamon-64bit.iso",
		"https://vault.centos.org/altarch/7.9.2009/isos/i386/sha256sum.txt":                     "CentOS-7-i386-Minimal-2009.iso",
		"https://iso.pop-os.org/22.04/amd64/intel/58/SHA256SUMS":                                "pop-os_22.04_amd64_intel_58.iso",
		"https://download.bazzite.gg/bazzite-deck-gnome-stable-live-amd64.iso-CHECKSUM":         "bazzite-deck-gnome-stable-live-amd64.iso",
		"https://mirror.cachyos.org/ISO/desktop/260809/cachyos-desktop-linux-260809.iso.sha256": "cachyos-desktop-linux-260809.iso",
	}
	for rawURL, name := range tests {
		body := remotetest.MustGet(t, rawURL)
		c, ok := Strongest(ParseManifest(body), name)
		if !ok || c.Algorithm != SHA256 || len(c.Hex) != 64 {
			t.Errorf("%s: got %v, %v; want a SHA-256 for %s", rawURL, c, ok, name)
		}
	}
}
