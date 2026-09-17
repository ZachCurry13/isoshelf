// Command isoshelf inventories and update-checks the bootable images in a
// folder: a Ventoy drive, a NAS share or Proxmox ISO storage.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	inv "github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/web"
)

// version is set by release builds: -ldflags "-X main.version=v0.1.0".
var version = "dev"

const usage = `isoshelf keeps the bootable images in a folder up to date.

Usage:
  isoshelf [ui] [flags] [folder]    open isoshelf in your web browser
  isoshelf scan  [flags] [folder]   list the images in a folder (offline)
  isoshelf check [flags] [folder]   list them and check for updates online
  isoshelf version                  print the version

The folder can be a Ventoy drive (like E:\ or /media/you/Ventoy), a folder on
a NAS share, or Proxmox ISO storage (like /var/lib/vz/template/iso). In
portable mode it defaults to the drive isoshelf runs from.

Flags for scan and check:
  --profile ventoy|proxmox  what kind of folder it is; remembered for next time
  --json                    print JSON instead of a table
  --catalog FILE            use this catalog instead of the built-in one
  --no-hash                 don't hash images whose filename never changes
  --no-update-check         don't check for a newer version of isoshelf

Flags for ui:
  --port N                  listen on this port (default: any free port)
  --no-browser              don't open the browser; just print the link
  --catalog FILE            use this catalog instead of the built-in one

isoshelf only writes to the .isoshelf folder inside the folder you give it,
and to its own settings folder. Set GITHUB_TOKEN to raise GitHub's rate limit.
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

type options struct {
	online        bool
	folder        string
	profile       string
	json          bool
	catalog       string
	noHash        bool
	noUpdateCheck bool
	port          int
	noBrowser     bool
}

// parseFlags reads flags and the folder, in any order.
func parseFlags(cmd string, args []string, stderr io.Writer) (options, error) {
	opts := options{online: cmd == "check"}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.catalog, "catalog", "", "")
	if cmd == "ui" {
		fs.IntVar(&opts.port, "port", 0, "")
		fs.BoolVar(&opts.noBrowser, "no-browser", false, "")
	} else {
		fs.StringVar(&opts.profile, "profile", "", "")
		fs.BoolVar(&opts.json, "json", false, "")
		fs.BoolVar(&opts.noHash, "no-hash", false, "")
		fs.BoolVar(&opts.noUpdateCheck, "no-update-check", false, "")
	}

	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				fmt.Fprint(stderr, usage)
			}
			return opts, err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	switch len(positional) {
	case 0:
	case 1:
		opts.folder = positional[0]
	default:
		return opts, fmt.Errorf("give one folder, not %d", len(positional))
	}
	if opts.profile != "" {
		if _, err := scan.ParseProfile(opts.profile); err != nil {
			return opts, err
		}
	}
	if opts.port < 0 || opts.port > 65535 {
		return opts, fmt.Errorf("port %d is out of range", opts.port)
	}
	return opts, nil
}

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
	cat, err := loadCatalog(dirs, opts.catalog)
	if err != nil {
		return err
	}

	client := remote.New(version)
	client.HTTP = e.http
	client.GitHubToken = e.getenv("GITHUB_TOKEN")

	// Ask GitHub about new isoshelf releases while the scan runs.
	notices := make(chan *appupdate.Notice, 1)
	if opts.online && !opts.noUpdateCheck && e.getenv("ISOSHELF_NO_UPDATE_CHECK") == "" {
		go func() {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			n, _ := appupdate.Check(ctx, client, dirs.Config, version, e.now())
			notices <- n
		}()
	} else {
		notices <- nil
	}

	res, err := inv.Run(ctx, inv.Options{
		Target:   target,
		Profile:  scan.Profile(opts.profile),
		Online:   opts.online,
		Client:   client,
		NoHash:   opts.noHash,
		Catalog:  cat,
		Dirs:     dirs,
		Now:      e.now,
		Progress: func(p inv.Progress) { progress(e, opts, p) },
	})
	progress(e, opts, inv.Progress{})
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

// serveUI runs the web UI on localhost until ctx is done.
func serveUI(ctx context.Context, e *env, opts options) error {
	dirs, err := findDirs(e)
	if err != nil {
		return err
	}
	cat, err := loadCatalog(dirs, opts.catalog)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.port)))
	if err != nil {
		return fmt.Errorf("can't listen on port %d (is isoshelf already running?): %w", opts.port, err)
	}
	token := rand.Text()
	url := fmt.Sprintf("http://%s/?token=%s", listener.Addr(), token)

	server := &http.Server{
		Handler: web.New(web.Config{
			Dirs:        dirs,
			Catalog:     cat,
			HTTP:        e.http,
			GitHubToken: e.getenv("GITHUB_TOKEN"),
			Version:     version,
			Token:       token,
			Target:      opts.folder,
			Now:         e.now,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	fmt.Fprintf(e.stdout, "isoshelf is running at:\n\n  %s\n\nKeep this window open while you use it. Press Ctrl+C to stop.\n", url)
	if e.listening != nil {
		e.listening(url)
	}
	if !opts.noBrowser {
		if err := e.openBrowser(url); err != nil {
			fmt.Fprintln(e.stderr, "isoshelf: couldn't open the browser; open the link above yourself:", err)
		}
	}

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
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

// loadCatalog loads the catalog named by the --catalog flag, else the user's
// copy in the config folder, else the built-in one.
func loadCatalog(dirs appdir.Dirs, flagPath string) (*catalog.Catalog, error) {
	name := flagPath
	if name == "" {
		name = filepath.Join(dirs.Config, "catalog.toml")
		if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
			return catalog.Default()
		}
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return catalog.Load(os.DirFS(filepath.Dir(abs)), filepath.Base(abs))
}
