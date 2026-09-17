package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// image is a stand-in for a downloadable image: big enough to arrive in
// several chunks.
var image = func() []byte {
	b := make([]byte, 3<<20)
	r := rand.NewChaCha8([32]byte{7})
	r.Read(b)
	return b
}()

var imageSHA256 = func() string {
	sum := sha256.Sum256(image)
	return hex.EncodeToString(sum[:])
}()

func testClient() *Client {
	c := New("test")
	c.Retries = 3
	c.Backoff = time.Millisecond
	c.ProgressEvery = 0
	return c
}

func sha256Checksum(hex string) *verify.Checksum {
	return &verify.Checksum{Name: "example.iso", Algorithm: verify.SHA256, Hex: hex}
}

// serveImage serves the image with range support.
func serveImage(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		http.ServeContent(w, r, "example.iso", time.Unix(0, 0), strings.NewReader(string(image)))
	}))
}

func request(dir, url string, checksum *verify.Checksum) Request {
	return Request{URLs: []string{url}, Filename: "example.iso", Dir: dir, Size: int64(len(image)), Checksum: checksum}
}

func TestDownload(t *testing.T) {
	server := serveImage(t)
	defer server.Close()
	dir := t.TempDir()

	var stages []Stage
	res, err := testClient().Download(context.Background(), request(dir, server.URL, sha256Checksum(imageSHA256)), func(p Progress) {
		if len(stages) == 0 || stages[len(stages)-1] != p.Stage {
			stages = append(stages, p.Stage)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Verified || res.SHA256 != imageSHA256 || res.Size != int64(len(image)) || res.Resumed {
		t.Errorf("result = %+v", res)
	}
	got, err := os.ReadFile(filepath.Join(dir, "example.iso"))
	if err != nil || string(got) != string(image) {
		t.Fatalf("placed file: %d bytes, %v", len(got), err)
	}
	if want := []Stage{Downloading, Verifying, Placing}; len(stages) != 3 || stages[0] != want[0] || stages[2] != want[2] {
		t.Errorf("stages = %v, want %v", stages, want)
	}
	// Nothing is left behind in the partial folder.
	if entries, _ := os.ReadDir(filepath.Join(dir, state.DirName, PartialDir)); len(entries) != 0 {
		t.Errorf("partial folder holds %d files", len(entries))
	}
}

func TestDownloadResumes(t *testing.T) {
	var calls atomic.Int32
	var ranges []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		if calls.Add(1) == 1 {
			// Send part of the file, then drop the connection.
			w.Header().Set("ETag", `"v1"`)
			w.Header().Set("Content-Length", "3145728")
			w.WriteHeader(http.StatusOK)
			w.Write(image[:1<<20])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		w.Header().Set("ETag", `"v1"`)
		http.ServeContent(w, r, "example.iso", time.Unix(0, 0), strings.NewReader(string(image)))
	}))
	defer server.Close()
	dir := t.TempDir()

	res, err := testClient().Download(context.Background(), request(dir, server.URL, sha256Checksum(imageSHA256)), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || res.SHA256 != imageSHA256 {
		t.Errorf("result = %+v, want a resumed download", res)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "example.iso")); err != nil || string(got) != string(image) {
		t.Fatalf("placed file: %d bytes, %v", len(got), err)
	}
	if len(ranges) != 2 || ranges[0] != "" || !strings.HasPrefix(ranges[1], "bytes=1048576-") {
		t.Errorf("ranges = %q, want the second request to resume", ranges)
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	server := serveImage(t)
	defer server.Close()
	dir := t.TempDir()

	wrong := strings.Repeat("a", 64)
	_, err := testClient().Download(context.Background(), request(dir, server.URL, sha256Checksum(wrong)), func(Progress) {})
	var mismatch *verify.Mismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("got %v, want a Mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "example.iso")); !os.IsNotExist(err) {
		t.Error("a file that failed its checksum was placed")
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, state.DirName, PartialDir)); len(entries) != 0 {
		t.Error("the bad download was kept")
	}
}

func TestDownloadUnverified(t *testing.T) {
	server := serveImage(t)
	defer server.Close()
	dir := t.TempDir()

	res, err := testClient().Download(context.Background(), request(dir, server.URL, nil), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verified || res.SHA256 != imageSHA256 {
		t.Errorf("result = %+v, want an unverified download with a hash", res)
	}
}

func TestDownloadMissingAndErrors(t *testing.T) {
	var calls atomic.Int32
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.NotFound(w, r)
	}))
	defer notFound.Close()
	dir := t.TempDir()

	_, err := testClient().Download(context.Background(), request(dir, notFound.URL, nil), func(Progress) {})
	var status *remote.StatusError
	if !errors.As(err, &status) || status.Code != http.StatusNotFound {
		t.Errorf("got %v, want a 404", err)
	}
	if calls.Load() != 1 {
		t.Errorf("a 404 was tried %d times, want 1", calls.Load())
	}

	if _, err := testClient().Download(context.Background(), Request{Dir: dir}, func(Progress) {}); err == nil {
		t.Error("no URLs: want an error")
	}
	bad := request(dir, notFound.URL, nil)
	bad.Filename = "../escape.iso"
	if _, err := testClient().Download(context.Background(), bad, func(Progress) {}); err == nil {
		t.Error("a filename with a path: want an error")
	}
}

func TestDownloadMirrorsAndReplace(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusBadGateway)
	}))
	defer broken.Close()
	server := serveImage(t)
	defer server.Close()
	dir := t.TempDir()

	req := Request{
		URLs:     []string{broken.URL, server.URL},
		Filename: "example.iso",
		Dir:      dir,
		Checksum: sha256Checksum(imageSHA256),
	}
	client := testClient()
	client.Retries = 1
	res, err := client.Download(context.Background(), req, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != server.URL {
		t.Errorf("came from %q, want the mirror %q", res.URL, server.URL)
	}

	// The file is there now, so a second download needs Replace.
	if _, err := client.Download(context.Background(), req, func(Progress) {}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("got %v, want an \"already exists\" error", err)
	}
	req.Replace = true
	if _, err := client.Download(context.Background(), req, func(Progress) {}); err != nil {
		t.Errorf("replacing: %v", err)
	}

	// Without a checksum, mirrors are not used.
	req.Checksum, req.Replace = nil, true
	if _, err := client.Download(context.Background(), req, func(Progress) {}); err == nil {
		t.Error("unverified download from a mirror: want an error")
	}
}

func TestDownloadCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Length", "3145728")
		w.WriteHeader(http.StatusOK)
		w.Write(image[:1<<20])
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Second) // the download is cancelled long before this
		w.Write(image[1<<20:])
	}))
	defer server.Close()
	dir := t.TempDir()

	// Cancel once the first megabyte has actually been written.
	_, err := testClient().Download(ctx, request(dir, server.URL, sha256Checksum(imageSHA256)), func(p Progress) {
		if p.Done >= 1<<20 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	// What was downloaded is kept, so it can be resumed later.
	part := filepath.Join(dir, state.DirName, PartialDir, "example.iso.part")
	info, err := os.Stat(part)
	if err != nil || info.Size() == 0 {
		t.Fatalf("partial file: %v", err)
	}
	if _, err := os.Stat(part + ".json"); err != nil {
		t.Errorf("no sidecar for the unfinished download: %v", err)
	}
}
