package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	inv "github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/web"
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
	cat, _, err := loadCatalog(dirs, opts.catalog)
	if err != nil {
		return err
	}
	cat = withOwnImages(cat, dirs, e.stderr)

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
	cat, catSource, err := loadCatalog(dirs, opts.catalog)
	if err != nil {
		return err
	}
	cat = withOwnImages(cat, dirs, e.stderr)
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.port)))
	if err != nil {
		return fmt.Errorf("can't listen on port %d (is isoshelf already running?): %w", opts.port, err)
	}
	token := rand.Text()
	url := fmt.Sprintf("http://%s/?token=%s", listener.Addr(), token)

	server := &http.Server{
		Handler: web.New(web.Config{
			Dirs:          dirs,
			Catalog:       cat,
			CatalogSource: catSource,
			HTTP:          e.http,
			GitHubToken:   e.getenv("GITHUB_TOKEN"),
			Version:       version,
			Token:         token,
			Target:        opts.folder,
			Now:           e.now,
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
