// Command isoshelf inventories and update-checks the bootable images in a
// folder: a Ventoy drive, a NAS share or Proxmox ISO storage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
)

// version is set by release builds: -ldflags "-X main.version=v0.1.0".
var version = "dev"

const usage = `isoshelf keeps the bootable images in a folder up to date.

Usage:
  isoshelf [ui] [flags] [folder]    open isoshelf in your web browser
  isoshelf scan  [flags] [folder]   list the images in a folder (offline)
  isoshelf check [flags] [folder]   list them and check for updates online
  isoshelf version                  print the version

The folder can be a Ventoy drive (like E:\ or /media/you/Ventoy), any folder
of images (a NAS share, a downloads folder, card images waiting to be
written), or Proxmox ISO storage (like /var/lib/vz/template/iso). In portable
mode it defaults to the drive isoshelf runs from.

Flags for scan and check:
  --profile folder|ventoy|proxmox
                            what kind of folder it is; remembered for next time
  --json                    print JSON instead of a table
  --catalog FILE            use this catalog instead of the built-in one
  --no-hash                 don't hash images whose filename never changes
  --no-update-check         don't check for a newer version of isoshelf

Flags for ui:
  --port N                  listen on this port (default: any free port)
  --listen ADDRESS          listen on this address instead of localhost, so
                            the page can be opened from another machine. Then
                            the token in the link is the only thing keeping
                            anyone out: use it on a network you trust.
  --no-browser              don't open the browser; just print the link
  --catalog FILE            use this catalog instead of the built-in one

isoshelf only writes to the .isoshelf folder inside the folder you give it,
and to its own settings folder. Set GITHUB_TOKEN to raise GitHub's rate limit,
and ISOSHELF_TOKEN to fix the secret in the link instead of making a new one
each time isoshelf starts.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], &env{stdout: os.Stdout, stderr: os.Stderr, interactive: isTerminal(os.Stderr)}))
}

// isTerminal reports whether f is a terminal (or Windows console) rather than
// a file or pipe.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// env is everything run needs from the outside world, so tests can replace
// it.
type env struct {
	stdout, stderr io.Writer
	// interactive is set when a person is watching stderr, so progress can be
	// shown there.
	interactive bool
	dirs        *appdir.Dirs     // nil: find them from the executable
	http        *http.Client     // nil: the default client
	now         func() time.Time // nil: time.Now
	getenv      func(string) string
	// openBrowser opens a URL; nil means the system's browser.
	openBrowser func(url string) error
	// listening, if set, is told the UI's address once it listens.
	listening func(url string)
}

func run(ctx context.Context, args []string, e *env) int {
	if e.now == nil {
		e.now = time.Now
	}
	if e.getenv == nil {
		e.getenv = os.Getenv
	}
	if e.openBrowser == nil {
		e.openBrowser = openBrowser
	}

	cmd := "ui"
	if len(args) > 0 && (args[0] == "" || args[0][0] != '-') {
		switch args[0] {
		case "ui", "scan", "check":
			cmd, args = args[0], args[1:]
		case "version", "--version", "-version":
			fmt.Fprintln(e.stdout, "isoshelf", version)
			return 0
		case "help":
			fmt.Fprint(e.stdout, usage)
			return 0
		default:
			// A folder given without a command opens the UI on it, unless it's
			// clearly a mistyped command.
			if _, err := os.Stat(args[0]); err != nil {
				fmt.Fprintf(e.stderr, "isoshelf: unknown command %q\n\n%s", args[0], usage)
				return 2
			}
		}
	}

	opts, err := parseFlags(cmd, args, e.stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(e.stderr, "isoshelf:", err)
		return 2
	}
	if cmd == "ui" {
		err = serveUI(ctx, e, opts)
	} else {
		err = inventory(ctx, e, opts)
	}
	if err != nil {
		fmt.Fprintln(e.stderr, "isoshelf:", err)
		return 1
	}
	return 0
}
