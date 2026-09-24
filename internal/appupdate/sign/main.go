// Command sign signs a release's checksum file. The release workflow runs it
// with the private key from the repository's secrets:
//
//	go run ./internal/appupdate/sign dist/SHA256SUMS
//
// with RELEASE_SIGNING_KEY set, and it writes dist/SHA256SUMS.sig. Before
// writing, it checks its own signature against the public key built into
// isoshelf, so a release signed with a key that doesn't match fails here
// rather than on everybody's computer.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sign:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("name the checksum file to sign")
	}
	secret := os.Getenv("RELEASE_SIGNING_KEY")
	if secret == "" {
		return errors.New("RELEASE_SIGNING_KEY isn't set. Releases have to be signed, or isoshelf " +
			"won't install them; see internal/appupdate/keygen for setting the key up")
	}
	key, err := appupdate.ParsePrivateKey(secret)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	sig := appupdate.Sign(key, data)
	if err := appupdate.VerifyRelease(data, sig); err != nil {
		return fmt.Errorf("the signature doesn't match the public key isoshelf is built with "+
			"(internal/appupdate/release.pub), so no isoshelf would install this release: %w", err)
	}
	return os.WriteFile(args[0]+appupdate.SignatureSuffix, []byte(sig), 0o644)
}
