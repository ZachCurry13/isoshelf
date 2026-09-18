package web

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/drives"
	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// maxBrowseEntries limits how many subfolders one listing returns.
const maxBrowseEntries = 2000

type folderJSON struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Label is what the operating system calls a drive, such as "Ventoy".
	Label string `json:"label,omitempty"`
	// Images counts the image files at the top level, or -1 when it wasn't
	// worked out: an unreadable folder, or one that took too long to answer.
	Images int `json:"images"`
}

type browseJSON struct {
	Path             string       `json:"path"`
	Parent           string       `json:"parent,omitempty"`
	Folders          []folderJSON `json:"folders"`
	Roots            []folderJSON `json:"roots"`
	SuggestedProfile string       `json:"suggested_profile"`
	// Images counts the image files in this folder, so the chooser can say
	// whether the right one has been found.
	Images int    `json:"images"`
	Error  string `json:"error,omitempty"`
}

// browse lists the subfolders of a folder, so the page can offer a folder
// picker: browsers can't show the computer's folders by themselves.
func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		s.mu.Lock()
		path = s.target
		s.mu.Unlock()
	}
	out := browseJSON{Folders: []folderJSON{}, Roots: roots()}
	if path == "" {
		writeJSON(w, http.StatusOK, out)
		return
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	out.Path, out.SuggestedProfile = abs, string(scan.SuggestProfile(abs))
	out.Images = countImages(abs)
	if parent := filepath.Dir(abs); parent != abs {
		out.Parent = parent
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		out.Error = "Can't open this folder: " + err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	for _, e := range entries {
		name := e.Name()
		if hiddenFolder(name) {
			continue
		}
		full := filepath.Join(abs, name)
		if !e.IsDir() {
			// Follow links to folders, such as Proxmox storage mounts.
			if e.Type()&os.ModeSymlink == 0 {
				continue
			}
			if info, err := os.Stat(full); err != nil || !info.IsDir() {
				continue
			}
		}
		out.Folders = append(out.Folders, folderJSON{Name: name, Path: full, Images: -1})
		if len(out.Folders) == maxBrowseEntries {
			break
		}
	}
	slices.SortFunc(out.Folders, func(a, b folderJSON) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	writeJSON(w, http.StatusOK, out)
}

func hiddenFolder(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") ||
		strings.EqualFold(name, "System Volume Information")
}

// roots are good places to start browsing: drives on Windows, and the usual
// mount points elsewhere. Each is asked what it's called and how many images
// it holds, which is what tells one drive letter from another.
func roots() []folderJSON {
	var out []folderJSON
	if runtime.GOOS == "windows" {
		for letter := 'A'; letter <= 'Z'; letter++ {
			drive := string(letter) + `:\`
			if _, err := os.Stat(drive); err == nil {
				out = append(out, folderJSON{Name: string(letter) + ":", Path: drive})
			}
		}
	} else {
		for _, p := range []string{"/", "/media", "/run/media", "/mnt", "/mnt/pve", "/var/lib/vz/template/iso"} {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				out = append(out, folderJSON{Name: p, Path: p})
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, folderJSON{Name: "Home", Path: home})
	}
	describe(out)
	return out
}

// describeWait is how long the whole picker waits for drives to answer. A
// network drive that has gone to sleep can take many seconds, and a chooser
// that hangs is worse than one that shows a drive letter on its own.
const describeWait = 1500 * time.Millisecond

// answer is one folder's description, and whether it arrived in time. The
// flag is what makes reading the other fields safe: they are only read once
// it is set, and a straggler that sets it later is never read at all.
type answer struct {
	label  string
	images int
	ready  atomic.Bool
}

// describe fills in each folder's label and image count, in parallel, giving
// up on the ones that are too slow. Anything unanswered keeps Images -1,
// which the page shows as nothing rather than as "no images".
func describe(folders []folderJSON) {
	answers := make([]answer, len(folders))
	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := range folders {
		folders[i].Images = -1
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			label, images := drives.Label(folders[i].Path), countImages(folders[i].Path)
			answers[i].label, answers[i].images = label, images
			answers[i].ready.Store(true)
		}(i)
	}
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(describeWait):
	}
	for i := range folders {
		if answers[i].ready.Load() {
			folders[i].Label, folders[i].Images = answers[i].label, answers[i].images
		}
	}
}

// maxCount stops counting a folder that holds thousands of files: the picker
// only needs enough to tell folders apart.
const maxCount = 500

// countImages counts the image files directly in dir, or -1 if it can't be
// read.
func countImages(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return -1
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if slices.Contains(catalog.ImageExtensions, strings.ToLower(filepath.Ext(e.Name()))) {
			count++
			if count == maxCount {
				break
			}
		}
	}
	return count
}
