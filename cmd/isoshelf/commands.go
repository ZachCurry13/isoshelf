package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	inv "github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/lastcheck"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// inventory runs a scan, and a check when opts.online is set, and prints the
// report.
func inventory(ctx context.Context, e *env, opts options) error {
	dirs, err := findDirs(e)
	if err != nil {
		return err
	}
	target := opts.folder
	if target == "" {
		if !dirs.Portable {
			cmd := "scan"
			if opts.online {
				cmd = "check"
			}
			return fmt.Errorf(`which folder? For example: isoshelf %s E:\`, cmd)
		}
		target = dirs.DefaultTarget
	}
	cat, _, err := loadCatalog(dirs, opts.catalog, e.stderr)
	if err != nil {
		return err
	}
	cat = withOwnImages(cat, dirs, e.stderr)

	client := remote.New(version)
	client.HTTP = e.http
	client.GitHubToken = e.getenv("GITHUB_TOKEN")

	// Ask GitHub about new isoshelf releases while the scan runs.
	notices := make(chan *appupdate.Notice, 1)
	wanted := settings.On(settings.Load(dirs.Config).AppUpdateCheck)
	if opts.online && wanted && !opts.noUpdateCheck && e.getenv("ISOSHELF_NO_UPDATE_CHECK") == "" {
		go func() {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			n, _ := appupdate.Check(ctx, client, dirs.Config, version, e.now())
			notices <- n
		}()
	} else {
		notices <- nil
	}

	// Typing "isoshelf check" is asking, so every project is asked however
	// recently it answered. The answers are still written down, which saves
	// the page from asking again when it opens.
	answers := lastcheck.Load(dirs.Config)
	answers.Now = e.now

	// The page and the command line share one settings file, so a folder
	// whose records were moved on the page is read from the same place here.
	records := state.Home(settings.Load(dirs.Config).RecordsHome(target, dirs.Config))

	res, err := inv.Run(ctx, inv.Options{
		Target:   target,
		Profile:  scan.Profile(opts.profile),
		Online:   opts.online,
		Memory:   answers.Asking(),
		Client:   client,
		NoHash:   opts.noHash,
		Catalog:  cat,
		Dirs:     dirs,
		Records:  records,
		Now:      e.now,
		Progress: func(p inv.Progress) { progress(e, opts, p) },
	})
	progress(e, opts, inv.Progress{})
	if opts.online {
		answers.Save() // best effort: the worst case is asking again
	}
	if res == nil {
		return err
	}

	notice := <-notices
	if opts.json {
		if err := writeJSON(e.stdout, res.Report, notice); err != nil {
			return err
		}
	} else {
		writeTable(e.stdout, res.Report)
		if notice != nil {
			fmt.Fprintln(e.stderr, "\n"+notice.String())
		}
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(e.stderr, "isoshelf:", w)
	}
	if err != nil {
		return errors.New("interrupted; results so far are saved")
	}
	return nil
}

// progress shows a one-line status on stderr, replacing the previous one. An
// empty Progress clears it. It stays quiet unless a person is watching and
// the output is a table.
func progress(e *env, opts options, p inv.Progress) {
	if opts.json || !e.interactive {
		return
	}
	var msg string
	switch p.Stage {
	case inv.Scanning:
		msg = "Scanning..."
	case inv.Hashing:
		msg = "Hashing " + p.File
		if p.Total > 0 {
			msg += fmt.Sprintf(": %d%%", p.Done*100/p.Total)
		}
	case inv.Checking:
		msg = "Checking for updates..."
		if p.Total > 0 {
			msg = fmt.Sprintf("Checking for updates (%d of %d)...", p.Done, p.Total)
		}
	case inv.Saving:
		msg = "Saving..."
	}
	fmt.Fprintf(e.stderr, "\r%-78.78s\r%s", "", msg)
}

// tokenFileName is where a server keeps the secret in its link.
const tokenFileName = "token"

// linkToken decides the secret in the link.
//
// ISOSHELF_TOKEN wins, for anyone who wants to choose it. Otherwise a desktop
// gets a fresh one every run: the browser opens with it and nothing has to
// remember it. A server keeps one in its config folder instead, because it
// restarts - when the machine reboots, when the image is updated - and a link
// somebody bookmarked should still work afterwards.
//
// Making it rather than asking for it is deliberate. An installer with a box
// marked "token" gets "password" typed into it, and that box is the only
// thing standing between a stranger on the network and somebody's images.
func linkToken(e *env, dirs appdir.Dirs, server bool) (string, error) {
	if t := e.getenv("ISOSHELF_TOKEN"); t != "" {
		if len(t) < 16 {
			return "", errors.New("ISOSHELF_TOKEN is too short to be a secret: use at least 16 characters, or leave it unset and isoshelf will make one for you")
		}
		return t, nil
	}
	if !server {
		return rand.Text(), nil
	}

	path := filepath.Join(dirs.Config, tokenFileName)
	if b, err := os.ReadFile(path); err == nil {
		if saved := strings.TrimSpace(string(b)); len(saved) >= 16 {
			return saved, nil
		}
	}
	token := rand.Text()
	if err := writeTokenFile(path, token); err != nil {
		// Not worth refusing to start over. isoshelf works; the link just
		// changes the next time it restarts, and the log says so.
		fmt.Fprintf(e.stderr, "isoshelf: couldn't save the link's token to %s, so it will be a different link after a restart: %v\n", path, err)
	}
	return token, nil
}

func writeTokenFile(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Readable only by the user isoshelf runs as: it is a password.
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

// isLoopback reports whether an address only the same machine can reach.
// Anything else - 0.0.0.0, a LAN address, a container's interface - means
// other machines can reach the page, which changes what the server accepts.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// openBrowser opens url in the system's default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}

func findDirs(e *env) (appdir.Dirs, error) {
	if e.dirs != nil {
		return *e.dirs, nil
	}
	return appdir.Find()
}
