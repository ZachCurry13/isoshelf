package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/remote"
)

// A download server that asks isoshelf to come back in an hour is asked once,
// not four times in thirty seconds.
func TestADownloadServerThatSaysComeBackLaterIsLeftAlone(t *testing.T) {
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := testClient().Download(context.Background(), request(t.TempDir(), srv.URL+"/example.iso", sha256Checksum(imageSHA256)), nil)
	var busy *remote.BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("got %v, want a BusyError", err)
	}
	if n := asked.Load(); n != 1 {
		t.Errorf("asked %d times, want once", n)
	}
}
