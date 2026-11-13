package web

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

// Make bootable (#3) from the page: a FydeOS .bin renamed at once, an Ubuntu
// Core .img.xz unpacked through the downloads with the original archived.
func TestMakeBootable(t *testing.T) {
	dir := t.TempDir()
	diskImage := bytes.Repeat([]byte("disk image "), 50000)
	diskImage[510], diskImage[511] = 0x55, 0xaa
	if err := os.WriteFile(filepath.Join(dir, "FydeOS_for_PC_iris_v22.0-SP1-io.bin"), diskImage, 0o644); err != nil {
		t.Fatal(err)
	}
	var packed bytes.Buffer
	w, _ := xz.NewWriter(&packed)
	w.Write(diskImage)
	w.Close()
	if err := os.WriteFile(filepath.Join(dir, "ubuntu-core-26-amd64.img.xz"), packed.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	plan := decode[struct{ Kind, To string }](t, request(t, s, http.MethodPost, "/api/fixup/plan", map[string]string{"path": "FydeOS_for_PC_iris_v22.0-SP1-io.bin"}))
	if plan.Kind != "rename" || plan.To != "FydeOS_for_PC_iris_v22.0-SP1-io.img" {
		t.Errorf("plan = %+v", plan)
	}
	if rec := request(t, s, http.MethodPost, "/api/fixup", map[string]string{"path": "FydeOS_for_PC_iris_v22.0-SP1-io.bin"}); rec.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}
	waitIdle(t, s)
	if _, err := os.Stat(filepath.Join(dir, "FydeOS_for_PC_iris_v22.0-SP1-io.img")); err != nil {
		t.Error("not renamed")
	}

	if rec := request(t, s, http.MethodPost, "/api/fixup", map[string]string{"path": "ubuntu-core-26-amd64.img.xz", "original": "delete"}); rec.Code != http.StatusBadRequest {
		t.Errorf("deleting the original was accepted: %d", rec.Code)
	}
	if rec := request(t, s, http.MethodPost, "/api/fixup", map[string]string{"path": "ubuntu-core-26-amd64.img.xz", "original": "move-aside"}); rec.Code != http.StatusAccepted {
		t.Fatalf("extract: %d %s", rec.Code, rec.Body)
	}
	waitIdle(t, s)
	if got, _ := os.ReadFile(filepath.Join(dir, "ubuntu-core-26-amd64.img")); !bytes.Equal(got, diskImage) {
		t.Error("the unpacked image isn't what was packed")
	}
	if _, err := os.Stat(filepath.Join(dir, "ubuntu-core-26-amd64.img.xz")); !os.IsNotExist(err) {
		t.Error("the compressed file wasn't archived")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range s.report.Items {
		if it.Path == "ubuntu-core-26-amd64.img" && it.Status == "not bootable" {
			t.Error("the unpacked image is still called not bootable")
		}
	}
}

// A file whose content doesn't back up the fix is refused, with a reason.
func TestMakeBootableRefusesWhatTheContentDoesntBackUp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "FydeOS_for_PC_iris_v22.0-SP1-io.bin"), []byte("not a disk at all"), 0o644)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)
	if rec := request(t, s, http.MethodPost, "/api/fixup/plan", map[string]string{"path": "FydeOS_for_PC_iris_v22.0-SP1-io.bin"}); rec.Code != http.StatusConflict {
		t.Errorf("plan answered %d %s, want a refusal", rec.Code, rec.Body)
	}
}
