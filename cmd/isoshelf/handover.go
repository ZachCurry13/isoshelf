package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
)

// Updating isoshelf's own program ends here: the old program stops
// listening and starts the new one in its place, on the same port, with the
// same link. The new one keeps the update once it is serving, or puts the old
// program back and starts that instead if it can't come up.

// uiServer is the page's HTTP server, which an update stops and, if handing
// over fails, starts again.
type uiServer struct {
	handler http.Handler
	addr    string
	errs    chan error
	// done is closed once a new program has taken over this one's window.
	done chan struct{}

	mu  sync.Mutex
	srv *http.Server
}

func (u *uiServer) start(l net.Listener) {
	srv := &http.Server{Handler: u.handler, ReadHeaderTimeout: 10 * time.Second}
	u.mu.Lock()
	u.srv = srv
	u.mu.Unlock()
	go func() {
		if err := srv.Serve(l); !errors.Is(err, http.ErrServerClosed) {
			select {
			case u.errs <- err:
			default:
			}
		}
	}()
}

func (u *uiServer) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u.mu.Lock()
	srv := u.srv
	u.mu.Unlock()
	return srv.Shutdown(ctx)
}

// handOver stops listening, so the port is free, and starts program in this
// one's place. On Linux that replaces this process and never returns; on
// Windows the new program starts in this window and this one exits. If it
// fails, the page is served again from here as if nothing had happened.
func (u *uiServer) handOver(program string, h appupdate.Handover, token string) error {
	u.shutdown()
	handedOver, err := appupdate.Restart(program, os.Args[1:], h.Environ(os.Environ(), token))
	if err == nil && handedOver {
		close(u.done)
		return nil
	}
	if err == nil {
		err = errors.New("it didn't start")
	}
	l, lerr := net.Listen("tcp", u.addr)
	if lerr != nil {
		u.errs <- fmt.Errorf("the new isoshelf didn't start (%v), and this one couldn't listen again: %w", err, lerr)
		return err
	}
	u.start(l)
	return err
}

// listenWait is how long a handed-over program tries for its port before it
// gives up and undoes the update. A test build shortens it with
// -ldflags "-X main.listenWait=2s".
var listenWait = "15s"

// listen listens on host and port. Straight after a handover the old program
// may still be letting go of the port, so a handed-over program tries for a
// while before giving up.
func listen(host string, port int, handed bool) (net.Listener, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	wait, err := time.ParseDuration(listenWait)
	if err != nil {
		wait = 15 * time.Second
	}
	deadline := time.Now().Add(wait)
	for {
		l, err := net.Listen("tcp", addr)
		if err == nil || !handed || time.Now().After(deadline) {
			return l, err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// keepUpdate keeps an update once the new program has been serving for a
// moment: the old programs set aside and the staging folder go.
func keepUpdate(e *env, h appupdate.Handover, confirmed *atomic.Bool) {
	time.Sleep(2 * time.Second)
	confirmed.Store(true)
	w, err := appupdate.LoadSwapped(h.Stage)
	if err != nil {
		fmt.Fprintf(e.stderr, "isoshelf: couldn't tidy up after the update (%v); the old program is still beside this one as a .old file.\n", err)
		return
	}
	w.Confirm(filepath.Dir(h.Stage))
}

// rollBack puts the old program back and starts it, because the new one
// can't come up. The old one is told why, and the page says so.
func rollBack(e *env, h appupdate.Handover, cause error) error {
	w, err := appupdate.LoadSwapped(h.Stage)
	if err != nil {
		return fmt.Errorf("%w; the update couldn't be undone either: %v", cause, err)
	}
	old := w.Previous()
	if err := w.Restore(); err != nil {
		return fmt.Errorf("%w; putting the old program back failed: %v", cause, err)
	}
	back := appupdate.Handover{
		Port:   h.Port,
		Failed: fmt.Sprintf("isoshelf %s didn't start (%v), so this is %s again.", w.To, cause, w.From),
	}
	fmt.Fprintln(e.stderr, "isoshelf:", back.Failed)
	if _, err := appupdate.Restart(old, os.Args[1:], back.Environ(os.Environ(), e.getenv("ISOSHELF_TOKEN"))); err != nil {
		return fmt.Errorf("%w; the old program is back but couldn't be started: %v", cause, err)
	}
	return nil
}
