package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

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
	// listen is the address the web UI binds to. Empty means 127.0.0.1: the
	// page is for the person at the keyboard. Anything else is server mode,
	// which is a deliberate choice and changes what the server accepts.
	listen string
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
		fs.StringVar(&opts.listen, "listen", "", "")
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
