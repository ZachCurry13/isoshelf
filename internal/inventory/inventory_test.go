package inventory

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// What Interim hands over is the caller's to keep and read while the run
// goes on hashing. It used to be the run's own working state, so the page
// read records a scan was still writing: a data race the race detector
// caught now and then in the web tests, and in the program a page refresh
// during a scan reading a map mid-write. Now it is a copy, and hashing
// afterwards leaves it as it was handed over.
func TestInterimIsTheCallersOwnCopy(t *testing.T) {
	dir := t.TempDir()
	for _, f := range sampledrive.Files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), []byte("stand-in"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}

	var handed *Result
	var reading sync.WaitGroup
	stop := make(chan struct{})
	res, err := Run(context.Background(), Options{
		Target:  dir,
		Profile: scan.Folder,
		Catalog: cat,
		HashAll: true,
		Interim: func(r *Result) {
			handed = r
			// Read it the way the page does, for as long as the run goes on.
			reading.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
					}
					for _, rec := range r.State.Files {
						_ = rec.SHA256
					}
					for _, it := range r.Report.Items {
						_ = it.Path
					}
				}
			})
		},
	})
	close(stop)
	reading.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if handed == nil {
		t.Fatal("Interim was never called")
	}

	hashedAfter := 0
	for _, rec := range res.State.Files {
		if rec.SHA256 != "" {
			hashedAfter++
		}
	}
	if hashedAfter == 0 {
		t.Fatal("the run hashed nothing, so this test proves nothing")
	}
	for path, rec := range handed.State.Files {
		if rec.SHA256 != "" {
			t.Errorf("%s gained a hash after it was handed over: the run is still writing the caller's copy", path)
		}
	}
}
