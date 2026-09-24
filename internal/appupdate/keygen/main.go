// Command keygen makes the key isoshelf releases are signed with. It is run
// once, by the maintainer, from the repository's folder:
//
//	go run ./internal/appupdate/keygen -private release-key.txt
//
// It writes the public half into internal/appupdate/release.pub, which every
// build of isoshelf carries, and the private half to the file named. That file
// is the secret: it goes into the repository's secrets as RELEASE_SIGNING_KEY
// and into a password manager as a backup, and is then deleted. Anyone with
// it can make an update every isoshelf will install.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
)

func main() {
	private := flag.String("private", "", "the file to write the private key to (it must not exist yet)")
	public := flag.String("public", "internal/appupdate/release.pub", "the public key file isoshelf is built with")
	replace := flag.Bool("replace", false, "replace a key that is already set up")
	flag.Parse()
	if err := run(*private, *public, *replace); err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
}

func run(private, public string, replace bool) error {
	if private == "" {
		return errors.New("name the file for the private key: -private release-key.txt")
	}
	if old, err := os.ReadFile(public); err == nil && strings.TrimSpace(string(old)) != "" && !replace {
		return errors.New("a release key is already set up. A new one means every isoshelf " +
			"already installed can no longer update itself, and has to be downloaded by hand " +
			"once. Add -replace if that is what you want")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	// O_EXCL: never write over a file that might already hold a key.
	f, err := os.OpenFile(private, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("can't write the private key: %w", err)
	}
	if _, err := fmt.Fprintln(f, appupdate.EncodeKey(priv.Seed())); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.WriteFile(public, []byte(appupdate.EncodeKey(pub)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf(`Done. Two files:

  %s   the public key. Commit it; every isoshelf built from now on
                  checks updates against it.
  %s   the PRIVATE key. Nobody else may see it.

Next:
  1. Store the private key as the repository's secret RELEASE_SIGNING_KEY:
       gh secret set RELEASE_SIGNING_KEY < %s
  2. Keep a copy somewhere safe, such as your password manager. If it is
     lost, the next release has to use a new key, and every isoshelf already
     installed must be updated by hand once.
  3. Delete %s.
`, public, private, private, private)
	return nil
}
