package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/auth"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/peer"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// sharingServer is an isoshelf holding one real file, offering it to others.
// It returns the server, the file's name and its hash.
func sharingServer(t *testing.T, sharing bool) (*Server, string, string) {
	t.Helper()
	dir := t.TempDir()
	const body = "not really an image, but it hashes like one"
	name := "linuxmint-22.3-cinnamon-64bit.iso"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs := testDirs(t)
	if err := settings.Save(dirs.Config, settings.Settings{ShareImages: &sharing}); err != nil {
		t.Fatal(err)
	}
	// Only a server shares: a desktop isoshelf answers its own computer alone.
	s := newServer(t, dirs, dir)
	s.cfg.AnyHost = true
	// A scan is what puts the file in the records, hash and all, and the
	// records are what sharing answers from.
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	sum := sha256.Sum256([]byte(body))
	return s, name, hex.EncodeToString(sum[:])
}

// have asks the sharing endpoint the way another isoshelf would.
func have(t *testing.T, s *Server, name, sum string) (bool, int) {
	t.Helper()
	rec := request(t, s, http.MethodGet, "/api/share/have?name="+name+"&sha256="+sum, nil)
	if rec.Code != http.StatusOK {
		return false, rec.Code
	}
	var answer struct {
		Have bool `json:"have"`
	}
	json.Unmarshal(rec.Body.Bytes(), &answer)
	return answer.Have, rec.Code
}

// An isoshelf that is sharing says what it holds, and hands it over.
func TestSharingAnImage(t *testing.T) {
	s, name, sum := sharingServer(t, true)
	if s.st == nil || s.st.Files == nil {
		t.Fatal("the scan recorded nothing, so there is nothing to share")
	}
	if rec, ok := s.st.Files[name]; !ok || rec.SHA256 == "" {
		t.Skipf("the scan didn't hash %s, so this isoshelf has nothing to offer for it", name)
	}

	got, code := have(t, s, name, sum)
	if !got {
		t.Fatalf("a file it holds: have=%v, code=%d", got, code)
	}
	body := request(t, s, http.MethodGet, "/api/share/file/"+name+"?sha256="+sum, nil)
	if body.Code != http.StatusOK {
		t.Fatalf("fetching it: %d", body.Code)
	}
	fetched, _ := io.ReadAll(body.Body)
	onDisk, err := os.ReadFile(filepath.Join(s.target, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(fetched) != string(onDisk) {
		t.Error("what was served isn't what is on disk")
	}
}

// The hash has to match, not just the name. A stale copy under the same name
// is exactly what somebody is trying to replace - handing it back would make
// the update a no-op that looked like it worked.
func TestSharingRefusesAFileOfTheSameNameWithDifferentBytes(t *testing.T) {
	s, name, _ := sharingServer(t, true)
	wrong := hex.EncodeToString(sha256.New().Sum(nil))
	if got, _ := have(t, s, name, wrong); got {
		t.Error("it claimed to hold a file it doesn't have, going by name alone")
	}
	if rec := request(t, s, http.MethodGet, "/api/share/file/"+name+"?sha256="+wrong, nil); rec.Code != http.StatusNotFound {
		t.Errorf("fetching by name with the wrong hash: %d, want 404", rec.Code)
	}
}

// Off unless turned on. Sharing hands whole images to whoever can sign in,
// which is a different thing from letting them manage the folder.
func TestSharingIsOffUnlessAskedFor(t *testing.T) {
	s, name, sum := sharingServer(t, false)
	if _, code := have(t, s, name, sum); code != http.StatusForbidden {
		t.Errorf("asking an isoshelf that isn't sharing: %d, want 403", code)
	}
	if rec := request(t, s, http.MethodGet, "/api/share/file/"+name+"?sha256="+sum, nil); rec.Code != http.StatusForbidden {
		t.Errorf("fetching from one that isn't sharing: %d, want 403", rec.Code)
	}
	if got := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)); got.Peer.Sharing {
		t.Error("a fresh isoshelf says it is sharing")
	}
}

// Nothing outside the folder, and no path typed in that reaches out of it.
// The name is looked up in the records rather than joined onto the folder,
// so a path that isn't a record isn't a path at all.
func TestSharingNeverReachesOutOfTheFolder(t *testing.T) {
	s, _, sum := sharingServer(t, true)
	secret := filepath.Join(filepath.Dir(s.target), "not-yours.txt")
	if err := os.WriteFile(secret, []byte("private"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"../not-yours.txt",
		"..%2Fnot-yours.txt",
		"/etc/passwd",
		"not-yours.txt",
	} {
		if got, _ := have(t, s, name, sum); got {
			t.Errorf("it offered %q", name)
		}
		rec := request(t, s, http.MethodGet, "/api/share/file/"+name+"?sha256="+sum, nil)
		if rec.Code == http.StatusOK {
			t.Errorf("it served %q", name)
		}
	}
}

// Sharing is behind the same door as everything else: a stranger who can
// reach the address but can't sign in gets nothing.
func TestSharingStillNeedsSigningIn(t *testing.T) {
	s, name, sum := sharingServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/api/share/have?name="+name+"&sha256="+sum, nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req) // no cookie at all
	if rec.Code != http.StatusForbidden {
		t.Errorf("without signing in: %d, want 403", rec.Code)
	}
}

// The whole point, end to end: one isoshelf holding a file, another fetching
// it from there instead of the internet - and the bytes still checked against
// the project's own checksum on the way in.
func TestFetchingFromTheOtherIsoshelf(t *testing.T) {
	holder, name, sum := sharingServer(t, true)
	if rec, ok := holder.st.Files[name]; !ok || rec.SHA256 == "" {
		t.Fatalf("the sharing isoshelf didn't hash %s, so it has nothing to offer", name)
	}
	// The sharing isoshelf has a login, as one on a network does, and the
	// asking one signs in to it. That is the whole of the trust here.
	if err := auth.Set(holder.cfg.Dirs.Config, "nas", "a good long password"); err != nil {
		t.Fatal(err)
	}
	// A real HTTP server in front of it, because the asking side signs in,
	// carries a cookie and resumes - none of which a function call exercises.
	shared := httptest.NewServer(holder)
	defer shared.Close()

	client, err := peer.New(strings.TrimPrefix(shared.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	client.FolderID, client.FolderName = "ASKINGDRIVE", "the laptop"
	if err := client.SignIn(t.Context(), "nas", "a good long password"); err != nil {
		t.Fatalf("signing in to the other isoshelf: %v", err)
	}

	have, size, err := client.Have(t.Context(), name, sum)
	if err != nil {
		t.Fatalf("asking whether it has the file: %v", err)
	}
	if !have || size == 0 {
		t.Fatalf("have=%v size=%d, want it to hold the file", have, size)
	}

	// Fetch it the way a download does, session and all.
	fetcher := fetch.New("test")
	fetcher.HTTP = withJar(nil, client.HTTP.Jar)
	fetcher.HostHeaders = map[string]map[string]string{client.Address: client.Headers()}
	into := t.TempDir()
	got, err := fetcher.Download(t.Context(), fetch.Request{
		URLs:     []string{client.FileURL(name, sum)},
		Filename: name,
		Dir:      into,
		Checksum: &verify.Checksum{Name: name, Algorithm: verify.SHA256, Hex: sum},
	}, func(fetch.Progress) {})
	if err != nil {
		t.Fatalf("fetching from the other isoshelf: %v", err)
	}
	if !got.Verified {
		t.Error("the file arrived unverified; the checksum is what makes this safe")
	}
	here, err := os.ReadFile(filepath.Join(into, name))
	if err != nil {
		t.Fatal(err)
	}
	there, err := os.ReadFile(filepath.Join(holder.target, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(here) != string(there) {
		t.Error("what arrived isn't what the other isoshelf holds")
	}

	// And the isoshelf that shared it wrote down which drive took it, which
	// is what makes rebuilding that drive possible later.
	record, err := holder.servedRecord()
	if err != nil {
		t.Fatalf("nothing was written down: %v", err)
	}
	if len(record) != 1 {
		t.Fatalf("it remembers %d drives, want 1: %+v", len(record), record)
	}
	drive := record[0]
	if drive.ID != "ASKINGDRIVE" || drive.Name != "the laptop" {
		t.Errorf("it remembers %+v, want the asking drive's id and name", drive)
	}
	if len(drive.Files) != 1 || drive.Files[0].Name != name {
		t.Errorf("files taken: %+v, want just %s", drive.Files, name)
	}
	if drive.Bytes != int64(len(there)) {
		t.Errorf("bytes taken = %d, want %d", drive.Bytes, len(there))
	}
}

// Taking the same file twice is one line, not two: what matters is that the
// drive has it, not how often it asked.
func TestTheSameFileTwiceIsRememberedOnce(t *testing.T) {
	holder, name, sum := sharingServer(t, true)
	if rec, ok := holder.st.Files[name]; !ok || rec.SHA256 == "" {
		t.Skipf("%s wasn't hashed", name)
	}
	who := Served{ID: "DRIVE", Name: "a drive"}
	file := ServedFile{Name: name, SHA256: sum, Size: 10}
	holder.noteServed(who, file)
	holder.noteServed(who, file)

	record, err := holder.servedRecord()
	if err != nil {
		t.Fatal(err)
	}
	if len(record) != 1 || len(record[0].Files) != 1 {
		t.Fatalf("remembered %+v, want one drive with one file", record)
	}
	if record[0].Bytes != 10 {
		t.Errorf("bytes = %d, want 10 rather than double-counted", record[0].Bytes)
	}
}

// A desktop isoshelf answers its own computer alone, so it never shares -
// even with the switch left on from before the switch was hidden there.
func TestADesktopIsoshelfDoesNotShare(t *testing.T) {
	s, name, sum := sharingServer(t, true)
	s.cfg.AnyHost = false
	if got, code := have(t, s, name, sum); got {
		t.Errorf("a desktop isoshelf offered its file (answer %d)", code)
	}
	if s.sharing() {
		t.Error("a desktop isoshelf thinks it is sharing, so its scans hash every image for nothing")
	}
}
