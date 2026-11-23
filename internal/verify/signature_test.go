package verify_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/verify"
	"github.com/ZachCurry13/isoshelf/internal/verify/verifytest"
)

func TestSignatures(t *testing.T) {
	project, stranger := verifytest.NewKey("project"), verifytest.NewKey("stranger")
	kr, err := verify.ReadKeyring([]byte(project.Armored), []string{project.Fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	sums := []byte("0123abcd  example-1.iso\n")

	for _, armored := range []bool{true, false} {
		if err := kr.CheckDetached(bytes.NewReader(sums), project.Detached(sums, armored)); err != nil {
			t.Errorf("armored=%v: the project's own signature failed: %v", armored, err)
		}
		tampered := []byte("ffffffff  example-1.iso\n")
		if err := kr.CheckDetached(bytes.NewReader(tampered), project.Detached(sums, armored)); !errors.Is(err, verify.ErrBadSignature) {
			t.Errorf("armored=%v: a changed checksum file passed: %v", armored, err)
		}
		if err := kr.CheckDetached(bytes.NewReader(sums), stranger.Detached(sums, armored)); !errors.Is(err, verify.ErrBadSignature) {
			t.Errorf("armored=%v: somebody else's signature passed: %v", armored, err)
		}
	}

	signed := append([]byte("text nobody signed\n"), project.Clearsign(sums)...)
	got, err := kr.Clearsigned(signed)
	if err != nil || !bytes.Contains(got, []byte("example-1.iso")) || bytes.Contains(got, []byte("nobody signed")) {
		t.Errorf("clearsigned: %q, %v", got, err)
	}
	if _, err := kr.Clearsigned(stranger.Clearsign(sums)); !errors.Is(err, verify.ErrBadSignature) {
		t.Errorf("somebody else's clearsigned file passed: %v", err)
	}
	if _, err := kr.Clearsigned(sums); !errors.Is(err, verify.ErrBadSignature) {
		t.Errorf("an unsigned file passed as clearsigned: %v", err)
	}

	if _, err := verify.ReadKeyring([]byte(project.Armored), []string{stranger.Fingerprint}); err == nil {
		t.Error("a key file was accepted for a fingerprint it doesn't hold")
	}
}
