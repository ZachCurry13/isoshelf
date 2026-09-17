// Command isoshelf inventories and update-checks the bootable images in a
// folder: a Ventoy drive, a NAS share or Proxmox ISO storage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// version is set by release builds: -ldflags "-X main.version=v0.1.0".
var version = "dev"

const usage = `isoshelf keeps the bootable images in a folder up to date.

Usage:
  isoshelf scan  [flags] [folder]   list the images in a folder (offline)
  isoshelf check [flags] [folder]   list them and check for updates online
  isoshelf version                  print the version

The folder can be a Ventoy drive (like E:\ or /media/you/Ventoy), a folder on
a NAS share, or Proxmox ISO storage (like /var/lib/vz/template/iso). In
portable mode it defaults to the drive isoshelf runs from.

Flags:
  --profile ventoy|proxmox  what kind of folder it is; remembered for next time
  --json                    print JSON instead of a table
  --catalog FILE            use this catalog instead of the built-in one
  --no-hash                 don't hash images whose filename never changes
  --no-update-check         don't check for a newer version of isoshelf

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
}

func run(ctx context.Context, args []string, e *env) int {
	if e.now == nil {
		e.now = time.Now
	}
	if e.getenv == nil {
		e.getenv = os.Getenv
	}
	if len(args) == 0 {
		fmt.Fprint(e.stderr, usage)
		return 2
	}
	switch cmd := args[0]; cmd {
	case "scan", "check":
		opts, err := parseFlags(cmd, args[1:], e.stderr)
		if err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			fmt.Fprintln(e.stderr, "isoshelf:", err)
			return 2
		}
		if err := inventory(ctx, e, opts); err != nil {
			fmt.Fprintln(e.stderr, "isoshelf:", err)
			return 1
		}
		return 0
	case "version", "--version", "-version":
		fmt.Fprintln(e.stdout, "isoshelf", version)
		return 0
	case "help", "--help", "-help", "-h":
		fmt.Fprint(e.stdout, usage)
		return 0
	default:
		fmt.Fprintf(e.stderr, "isoshelf: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

type options struct {
	online        bool
	folder        string
	profile       string
	json          bool
	catalog       string
	noHash        bool
	noUpdateCheck bool
}

// parseFlags reads flags and the folder, in any order.
func parseFlags(cmd string, args []string, stderr io.Writer) (options, error) {
	opts := options{online: cmd == "check"}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.profile, "profile", "", "")
	fs.BoolVar(&opts.json, "json", false, "")
	fs.StringVar(&opts.catalog, "catalog", "", "")
	fs.BoolVar(&opts.noHash, "no-hash", false, "")
	fs.BoolVar(&opts.noUpdateCheck, "no-update-check", false, "")

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
	return opts, nil
}

// inventory runs a scan, and a check when opts.online is set.
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
	if target, err = filepath.Abs(target); err != nil {
		return err
	}
	cat, err := loadCatalog(dirs, opts.catalog)
	if err != nil {
		return err
	}

	// Ask GitHub about new isoshelf releases while the scan runs.
	client := remote.New(version)
	client.HTTP = e.http
	client.GitHubToken = e.getenv("GITHUB_TOKEN")
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

	st, err := state.Load(target)
	if err != nil {
		return err
	}
	if opts.profile != "" {
		st.Profile = scan.Profile(opts.profile)
	}
	res, err := scan.Scan(ctx, target, cat, scan.Options{Profile: st.Profile, Skip: []string{dirs.App}})
	if err != nil {
		return err
	}
	st.RecordScan(res, e.now())

	var hashErr error
	if !opts.noHash {
		hashErr = hash(ctx, e, opts, res, st, cat)
	}

	report := check.Offline(res, st, cat)
	if opts.online && ctx.Err() == nil {
		progress(e, opts, "Checking for updates...")
		report.Online(ctx, client, st)
		progress(e, opts, "")
	}

	if err := st.Save(target); err != nil {
		fmt.Fprintf(e.stderr, "isoshelf: couldn't save what it learned to %s: %v\n", filepath.Join(target, state.DirName), err)
	}
	if !dirs.Portable {
		if err := st.SaveMirror(dirs.Config, target, e.now()); err != nil {
			fmt.Fprintf(e.stderr, "isoshelf: couldn't save a copy of the history: %v\n", err)
		}
	}

	notice := <-notices
	if opts.json {
		err = writeJSON(e.stdout, report, notice)
	} else {
		writeTable(e.stdout, report)
		if notice != nil {
			fmt.Fprintln(e.stderr, "\n"+notice.String())
		}
	}
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return errors.New("interrupted; results so far are saved")
	}
	if hashErr != nil {
		fmt.Fprintln(e.stderr, "isoshelf: some images couldn't be hashed:", hashErr)
	}
	return nil
}

// hash computes the hashes fixed-name images need, showing progress.
func hash(ctx context.Context, e *env, opts options, res *scan.Result, st *state.State, cat *catalog.Catalog) error {
	files := st.NeedsHash(res, cat)
	for i := range files {
		err := st.HashFiles(ctx, res.Root, files[i:i+1], func(f scan.File, done int64) {
			if f.Size > 0 {
				progress(e, opts, fmt.Sprintf("Hashing %s (%d of %d): %d%%", f.Name(), i+1, len(files), done*100/f.Size))
			}
		})
		if err != nil {
			progress(e, opts, "")
			return err
		}
	}
	progress(e, opts, "")
	return nil
}

// progress shows a one-line status on stderr, replacing the previous one. An
// empty message clears it. It stays quiet unless a person is watching and the
// output is a table.
func progress(e *env, opts options, msg string) {
	if opts.json || !e.interactive {
		return
	}
	fmt.Fprintf(e.stderr, "\r%-78.78s\r%s", "", msg)
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
