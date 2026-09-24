package appupdate

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/remote"
)

// fakeRelease serves one release the way GitHub does: the list of its files,
// then each file. Nothing here goes near the network.
type fakeRelease struct {
	tag   string
	files map[string][]byte
	// sign signs SHA256SUMS; nil leaves the release unsigned.
	sign ed25519.PrivateKey
}

func (f *fakeRelease) serve(t *testing.T) (*httptest.Server, Source) {
	t.Helper()
	files := map[string][]byte{}
	var sums strings.Builder
	for name, body := range f.files {
		files[name] = body
		sum := sha256.Sum256(body)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	files[SumsFile] = []byte(sums.String())
	if f.sign != nil {
		files[SumsFile+SignatureSuffix] = []byte(Sign(f.sign, files[SumsFile]))
	}

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/releases/tags/", func(w http.ResponseWriter, r *http.Request) {
		var assets []asset
		for name, body := range files {
			assets = append(assets, asset{Name: name, URL: srv.URL + "/download/" + name, Size: int64(len(body))})
		}
		json.NewEncoder(w).Encode(map[string]any{"tag_name": f.tag, "assets": assets})
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[strings.TrimPrefix(r.URL.Path, "/download/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	})
	srv = httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	fc := fetch.New("test")
	fc.HTTP, fc.Retries = srv.Client(), 0
	return srv, Source{
		API:    srv.URL + "/releases/tags/",
		Client: &remote.Client{HTTP: srv.Client()},
		Fetch:  fc,
	}
}

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func ownSuffix(t *testing.T) string {
	t.Helper()
	s := Suffix(runtime.GOOS, runtime.GOARCH)
	if s == "" {
		t.Skip("isoshelf publishes no program for this system")
	}
	return s
}

// A signed release is downloaded, checked and left waiting beside the
// program, and the program itself isn't touched yet.
func TestPrepareDownloadsAndChecksASignedRelease(t *testing.T) {
	suffix := ownSuffix(t)
	pub, priv := testKey(t)
	rel := &fakeRelease{tag: "v9.0.0", sign: priv, files: map[string][]byte{
		"isoshelf-v9.0.0-" + suffix: []byte("the new program"),
	}}
	_, src := rel.serve(t)
	src.Key = pub

	dir := t.TempDir()
	exe := filepath.Join(dir, "isoshelf-"+suffix)
	os.WriteFile(exe, []byte("the old program"), 0o755)
	targets, err := Targets(exe, false)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := Prepare(context.Background(), src, "v9.0.0", targets, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(staged.Files[0].New); string(got) != "the new program" {
		t.Errorf("staged %q, want the new program", got)
	}
	if got, _ := os.ReadFile(exe); string(got) != "the old program" {
		t.Error("preparing an update changed the running program")
	}
}

// The one rule that matters: an update without the project's signature, or
// with somebody else's, is never kept.
func TestPrepareRefusesWhatIsNotSigned(t *testing.T) {
	suffix := ownSuffix(t)
	pub, _ := testKey(t)
	_, stranger := testKey(t)
	for _, c := range []struct {
		why  string
		sign ed25519.PrivateKey
	}{
		{"unsigned", nil},
		{"signed by someone else", stranger},
	} {
		rel := &fakeRelease{tag: "v9.0.0", sign: c.sign, files: map[string][]byte{
			"isoshelf-v9.0.0-" + suffix: []byte("somebody's program"),
		}}
		_, src := rel.serve(t)
		src.Key = pub
		dir := t.TempDir()
		exe := filepath.Join(dir, "isoshelf-"+suffix)
		os.WriteFile(exe, []byte("the old program"), 0o755)
		targets, _ := Targets(exe, false)
		if _, err := Prepare(context.Background(), src, "v9.0.0", targets, nil); err == nil {
			t.Errorf("%s: the update was accepted", c.why)
		} else if c.sign != nil && !errors.Is(err, ErrBadSignature) {
			t.Errorf("%s: %v, want ErrBadSignature", c.why, err)
		}
		if names, _ := os.ReadDir(filepath.Join(dir, StageDir)); len(names) > 0 {
			t.Errorf("%s: a refused update left %d files waiting", c.why, len(names))
		}
	}
}

// A signed checksum file vouches for the bytes: a program that doesn't match
// its line is refused like any other checksum mismatch.
func TestPrepareRefusesAProgramThatDoesNotMatch(t *testing.T) {
	suffix := ownSuffix(t)
	pub, priv := testKey(t)
	name := "isoshelf-v9.0.0-" + suffix
	rel := &fakeRelease{tag: "v9.0.0", sign: priv, files: map[string][]byte{name: []byte("the real program")}}
	srv, src := rel.serve(t)
	src.Key = pub
	// Swap the bytes after the checksums were worked out.
	srv.Config.Handler.(*http.ServeMux).HandleFunc("/download/"+name, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("a different program"))
	})
	dir := t.TempDir()
	exe := filepath.Join(dir, "isoshelf-"+suffix)
	os.WriteFile(exe, []byte("the old program"), 0o755)
	targets, _ := Targets(exe, false)
	if _, err := Prepare(context.Background(), src, "v9.0.0", targets, nil); err == nil {
		t.Fatal("a program that doesn't match the signed checksum was accepted")
	}
}

// A build made before the key was set up has nothing to check against, and
// so installs nothing.
func TestPrepareWithoutAKeyInstallsNothing(t *testing.T) {
	if _, ok := releaseKey(); ok {
		t.Skip("this build carries a release key")
	}
	_, err := Prepare(context.Background(), Source{}, "v9.0.0", []Target{{Path: "x"}}, nil)
	if !errors.Is(err, ErrNoKey) {
		t.Errorf("got %v, want ErrNoKey", err)
	}
}
