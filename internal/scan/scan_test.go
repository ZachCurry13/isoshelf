package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
)

// content returns small stand-in file contents that sniff as the given kind.
func content(kind string) []byte {
	switch kind {
	case "iso":
		b := make([]byte, 40000)
		copy(b[32769:], "CD001")
		return b
	case "disk":
		b := make([]byte, 1024)
		b[510], b[511] = 0x55, 0xAA
		copy(b[512:], "EFI PART")
		return b
	}
	return []byte("some notes\n")
}

// writeFiles creates files under dir. Keys are slash-separated paths.
func writeFiles(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func defaultCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// paths returns the paths of the files a scan found.
func paths(res *Result) []string {
	var out []string
	for _, f := range res.Files {
		out = append(out, f.Path)
	}
	return out
}

func TestScanSampleDrive(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{}
	for _, f := range sampledrive.Files {
		files[f.Name] = content(f.Content)
	}
	writeFiles(t, dir, files)

	res, err := Scan(context.Background(), dir, defaultCatalog(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Profile != Ventoy {
		t.Errorf("profile = %q, want %q", res.Profile, Ventoy)
	}
	if len(res.Problems) != 0 {
		t.Errorf("problems: %v", res.Problems)
	}

	found := map[string]File{}
	for _, f := range res.Files {
		found[f.Path] = f
	}
	for _, want := range sampledrive.Files {
		got, ok := found[want.Name]
		if want.Name == "notes.txt" {
			if ok {
				t.Errorf("notes.txt was listed")
			}
			continue
		}
		if !ok {
			t.Errorf("%s: not listed", want.Name)
			continue
		}

		switch {
		case want.Entry == "" && len(got.Matches) != 0:
			t.Errorf("%s: matched %q, want unrecognized", want.Name, got.Matches[0].Entry.ID)
		case want.Entry != "" && (len(got.Matches) != 1 || got.Matches[0].Entry.ID != want.Entry || got.Matches[0].Version != want.Version):
			t.Errorf("%s: matches %v, want %s version %q", want.Name, got.Matches, want.Entry, want.Version)
		}
		if got.Kind != sniff.Kind(want.Content) {
			t.Errorf("%s: kind %s, want %s", want.Name, got.Kind, want.Content)
		}
		wantBootable := !strings.HasSuffix(want.Name, ".bin")
		if got.Bootable != wantBootable {
			t.Errorf("%s: bootable = %v, want %v", want.Name, got.Bootable, wantBootable)
		}
		if got.Size != int64(len(files[want.Name])) {
			t.Errorf("%s: size %d, want %d", want.Name, got.Size, len(files[want.Name]))
		}
	}
	if n := len(sampledrive.Files) - 1; len(res.Files) != n {
		t.Errorf("listed %d files, want %d: %q", len(res.Files), n, paths(res))
	}
}

func TestScanProxmox(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string][]byte{
		"unknown-distro.ISO":                  content("iso"),
		"batocera-5.25-x86-20200310.img":      content("disk"),
		"FydeOS_for_PC_iris_v22.0-SP1-io.bin": content("disk"),
		"tools.wim":                           content("text"),
		"sub/netboot.xyz.iso":                 content("iso"),
	})

	res, err := Scan(context.Background(), dir, defaultCatalog(t), Options{Profile: Proxmox})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FydeOS_for_PC_iris_v22.0-SP1-io.bin", "batocera-5.25-x86-20200310.img", "unknown-distro.ISO"}
	if got := paths(res); !slices.Equal(got, want) {
		t.Fatalf("listed %q, want %q (top level only, no .wim)", got, want)
	}
	for _, f := range res.Files {
		if wantBootable := !strings.HasSuffix(f.Path, ".bin"); f.Bootable != wantBootable {
			t.Errorf("%s: bootable = %v, want %v", f.Path, f.Bootable, wantBootable)
		}
	}
}

func TestScanSkipsFolders(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string][]byte{
		"ventoy/ventoy.json":                                        []byte("{}"),
		"ventoy/netboot.xyz.iso":                                    content("iso"),
		".isoshelf/partial/netboot.xyz.iso":                         content("iso"),
		".Trash-1000/files/old.iso":                                 make([]byte, 1000),
		"$RECYCLE.BIN/S-1-5-21/old.iso":                             make([]byte, 500),
		"System Volume Information/tracking.log":                    []byte("x"),
		"isoshelf/netboot.xyz.iso":                                  content("iso"),
		"distros/ventoy/netboot.xyz.iso":                            content("iso"),
		"distros/linux/linuxmint-22.3-cinnamon-64bit.iso":           content("iso"),
		"distros/linux/.isoshelf/linuxmint-22.3-cinnamon-64bit.iso": content("iso"),
	})

	res, err := Scan(context.Background(), dir, defaultCatalog(t), Options{
		Skip: []string{filepath.Join(dir, "isoshelf")}, // the portable app folder
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"distros/linux/linuxmint-22.3-cinnamon-64bit.iso", "distros/ventoy/netboot.xyz.iso"}
	if got := paths(res); !slices.Equal(got, want) {
		t.Errorf("listed %q, want %q", got, want)
	}
	wantTrash := []Trash{{"$RECYCLE.BIN", 500}, {".Trash-1000", 1000}}
	slices.SortFunc(res.Trash, func(a, b Trash) int { return strings.Compare(a.Path, b.Path) })
	if !slices.Equal(res.Trash, wantTrash) {
		t.Errorf("trash = %v, want %v", res.Trash, wantTrash)
	}
}

func TestScanErrors(t *testing.T) {
	cat := defaultCatalog(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string][]byte{"netboot.xyz.iso": content("iso")})

	if _, err := Scan(context.Background(), filepath.Join(dir, "missing"), cat, Options{}); err == nil {
		t.Error("missing folder: want an error")
	}
	if _, err := Scan(context.Background(), filepath.Join(dir, "netboot.xyz.iso"), cat, Options{}); err == nil {
		t.Error("file instead of folder: want an error")
	}
	if _, err := Scan(context.Background(), dir, cat, Options{Profile: "usb"}); err == nil {
		t.Error("unknown profile: want an error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(ctx, dir, cat, Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled scan: got %v, want context.Canceled", err)
	}
}

func TestSuggestProfile(t *testing.T) {
	tests := map[string]Profile{
		"/var/lib/vz/template/iso":          Proxmox,
		"/mnt/pve/nas-isos/template/iso/":   Proxmox,
		`\\truenas\isos\template\iso`:       Proxmox,
		"E:\\":                              Ventoy,
		"/media/zach/Ventoy":                Ventoy,
		"/mnt/nas/isos":                     Ventoy,
		"/var/lib/vz/template/iso/archived": Ventoy,
	}
	for dir, want := range tests {
		if got := SuggestProfile(dir); got != want {
			t.Errorf("SuggestProfile(%q) = %q, want %q", dir, got, want)
		}
	}
}

// TestScanProxmoxFolder scans a copy of the maintainer's Proxmox ISO folder.
func TestScanProxmoxFolder(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{"Files/ubuntu-24.04.4-desktop-amd64.iso": content("iso")}
	for _, f := range sampledrive.ProxmoxFolder {
		files[f.Name] = content(f.Content)
	}
	writeFiles(t, dir, files)

	res, err := Scan(context.Background(), dir, defaultCatalog(t), Options{Profile: SuggestProfile(filepath.Join(dir, "template", "iso"))})
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]File{}
	for _, f := range res.Files {
		listed[f.Path] = f
	}
	for _, want := range sampledrive.ProxmoxFolder {
		ext := strings.ToLower(filepath.Ext(want.Name))
		wantListed := want.Entry != "" || ext == ".iso" || ext == ".img"
		got, ok := listed[want.Name]
		switch {
		case ok != wantListed:
			t.Errorf("%s: listed = %v, want %v", want.Name, ok, wantListed)
		case ok && got.Bootable != (ext == ".iso" || ext == ".img"):
			t.Errorf("%s: bootable = %v", want.Name, got.Bootable)
		}
	}
	if _, ok := listed["Files/ubuntu-24.04.4-desktop-amd64.iso"]; ok {
		t.Error("the Proxmox profile scanned a subfolder")
	}
}
