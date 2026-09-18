package scan

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
)

// Profile says what kind of target a folder is.
type Profile string

const (
	// Ventoy is a Ventoy drive: subfolders are scanned, and bootable means
	// an extension Ventoy lists.
	Ventoy Profile = "ventoy"
	// Proxmox is Proxmox VE ISO storage, which lists only .iso and .img files
	// at the top level of the folder.
	Proxmox Profile = "proxmox"
	// Folder is any other folder of images: a NAS share, a downloads folder,
	// an unRAID or TrueNAS share, a hypervisor's ISO library, or a stash of
	// card images waiting to be written. Subfolders are scanned, compressed
	// images count, and nothing is called "not bootable", because no boot
	// menu is reading this folder.
	Folder Profile = "folder"
)

// archiveSuffixes are compressed images, which a folder keeps but a boot menu
// can't read. They are matched as whole suffixes so that an ordinary .zip or
// .gz in a downloads folder is left alone.
var archiveSuffixes = []string{".img.xz", ".img.gz", ".img.zip", ".iso.xz", ".iso.gz", ".iso.zip"}

// ParseProfile checks a profile name.
func ParseProfile(s string) (Profile, error) {
	switch p := Profile(s); p {
	case Ventoy, Proxmox, Folder:
		return p, nil
	}
	return "", fmt.Errorf("unknown profile %q (use %q, %q or %q)", s, Ventoy, Proxmox, Folder)
}

// Extensions returns the lowercase extensions the profile lists.
func (p Profile) Extensions() []string {
	if p == Proxmox {
		return []string{".iso", ".img"}
	}
	return catalog.ImageExtensions
}

// Boots reports whether a boot menu reads this folder, and so whether a file
// it can't list is worth complaining about. A plain folder has no menu: an
// image there is on its way to a card or a stick.
func (p Profile) Boots() bool { return p != Folder }

// Recursive reports whether the profile scans subfolders.
func (p Profile) Recursive() bool {
	return p != Proxmox
}

// Lists reports whether the profile lists a file with this name.
func (p Profile) Lists(name string) bool {
	lower := strings.ToLower(name)
	if p == Folder {
		for _, suffix := range archiveSuffixes {
			if strings.HasSuffix(lower, suffix) {
				return true
			}
		}
	}
	return slices.Contains(p.Extensions(), path.Ext(lower))
}

// SuggestProfile guesses what kind of folder this is: Proxmox ISO storage by
// its path, a Ventoy drive by the ventoy folder Ventoy puts there, and a
// plain folder otherwise. The user can always say otherwise.
func SuggestProfile(dir string) Profile {
	d := strings.ToLower(strings.TrimRight(strings.ReplaceAll(dir, `\`, "/"), "/"))
	if d == "template/iso" || strings.HasSuffix(d, "/template/iso") {
		return Proxmox
	}
	if info, err := os.Stat(filepath.Join(dir, "ventoy")); err == nil && info.IsDir() {
		return Ventoy
	}
	return Folder
}
