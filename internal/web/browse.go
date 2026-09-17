package web

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// maxBrowseEntries limits how many subfolders one listing returns.
const maxBrowseEntries = 2000

type folderJSON struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type browseJSON struct {
	Path             string       `json:"path"`
	Parent           string       `json:"parent,omitempty"`
	Folders          []folderJSON `json:"folders"`
	Roots            []folderJSON `json:"roots"`
	SuggestedProfile string       `json:"suggested_profile"`
	Error            string       `json:"error,omitempty"`
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
		out.Folders = append(out.Folders, folderJSON{Name: name, Path: full})
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
// mount points elsewhere.
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
	return out
}
