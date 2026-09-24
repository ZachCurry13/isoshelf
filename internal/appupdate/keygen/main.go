// Command keygen makes the key isoshelf releases are signed with. It is run
// once, by the maintainer, from the repository's folder:
//
//	go run ./internal/appupdate/keygen -private release-key.txt -secret RELEASE_SIGNING_KEY
//
// It writes the public half into internal/appupdate/release.pub, which every
// build of isoshelf carries, and the private half to the file named. With
// -secret it also stores the private half as that GitHub secret, through the
// gh command, so it never has to be copied by hand. The file is then for a
// password manager, as the backup, and is deleted. Anyone with it can make an
// update every isoshelf will install.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
)

func main() {
	private := flag.String("private", "", "the file to write the private key to (it must not exist yet)")
	public := flag.String("public", "internal/appupdate/release.pub", "the public key file isoshelf is built with")
	secret := flag.String("secret", "", "also store the private key as this GitHub secret, using gh")
	replace := flag.Bool("replace", false, "replace a key that is already set up")
	flag.Parse()
	if err := run(*private, *public, *secret, *replace); err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
}

func run(private, public, secret string, replace bool) error {
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
	line := appupdate.EncodeKey(priv.Seed()) + "\n"
	// O_EXCL: never write over a file that might already hold a key.
	f, err := os.OpenFile(private, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("can't write the private key: %w", err)
	}
	if _, err := f.WriteString(line); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.WriteFile(public, []byte(appupdate.EncodeKey(pub)+"\n"), 0o644); err != nil {
		return err
	}

	stored := false
	if secret != "" {
		// The key goes to gh on its standard input, never on a command line
		// where other programs could see it.
		cmd := exec.Command("gh", "secret", "set", secret)
		cmd.Stdin = bytes.NewReader([]byte(line))
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "keygen: couldn't store the secret with gh (%v); store it by hand as step 1 below says.\n", err)
		} else {
			stored = true
		}
	}

	fmt.Printf("\nDone.\n\n  %s   the public key. It is committed with the release, and every\n"+
		"                  isoshelf built from now on checks updates against it.\n"+
		"  %s   the PRIVATE key. Nobody else may see it.\n\nNext:\n", public, private)
	step := 1
	if stored {
		fmt.Printf("  - The private key is stored as the GitHub secret %s.\n", secret)
	} else {
		fmt.Printf("  %d. Store the private key as the repository's secret RELEASE_SIGNING_KEY:\n"+
			"       gh secret set RELEASE_SIGNING_KEY < %s\n", step, private)
		step++
	}
	fmt.Printf("  %d. Keep a copy somewhere safe, such as your password manager. If it is\n"+
		"     lost, the next release has to use a new key, and every isoshelf already\n"+
		"     installed must be updated by hand once.\n"+
		"  %d. Delete %s.\n", step, step+1, private)
	return nil
}
