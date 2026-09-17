// Package scan walks a target folder and lists its image files: which catalog
// entry each belongs to, what its content looks like, and whether the target's
// boot menu or storage would list it.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
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
)

// ParseProfile checks a profile name.
func ParseProfile(s string) (Profile, error) {
	switch p := Profile(s); p {
	case Ventoy, Proxmox:
		return p, nil
	}
	return "", fmt.Errorf("unknown profile %q (use %q or %q)", s, Ventoy, Proxmox)
}

// Extensions returns the lowercase extensions the profile lists.
func (p Profile) Extensions() []string {
	if p == Proxmox {
		return []string{".iso", ".img"}
	}
	return catalog.ImageExtensions
}

// Recursive reports whether the profile scans subfolders.
func (p Profile) Recursive() bool {
	return p != Proxmox
}

// Lists reports whether the profile lists a file with this name.
func (p Profile) Lists(name string) bool {
	return slices.Contains(p.Extensions(), strings.ToLower(path.Ext(name)))
}

// SuggestProfile returns Proxmox for a folder that looks like Proxmox ISO
// storage (its path ends in template/iso), and Ventoy otherwise.
func SuggestProfile(dir string) Profile {
	d := strings.ToLower(strings.TrimRight(strings.ReplaceAll(dir, `\`, "/"), "/"))
	if d == "template/iso" || strings.HasSuffix(d, "/template/iso") {
		return Proxmox
	}
	return Ventoy
}

// Options adjust a scan.
type Options struct {
	// Profile defaults to Ventoy.
	Profile Profile
	// Skip lists more folders to leave out, such as the portable app folder.
	Skip []string
}

// Result is what a scan found.
type Result struct {
	Root    string // absolute path of the scanned folder
	Profile Profile
	// Files are the images, sorted by path.
	Files []File
	// Trash lists trash folders and how much space they use.
	Trash []Trash
	// Problems are files or folders that couldn't be read. The scan skipped
	// them and went on.
	Problems []Problem
}

// File is an image file, or a file the catalog recognizes by name.
type File struct {
	Path    string // relative to Result.Root, with / separators
	Size    int64
	ModTime time.Time
	Kind    sniff.Kind
	// Bootable reports whether the profile lists the file's extension.
	Bootable bool
	// Matches are the catalog entries whose pattern matches the filename.
	// None means unrecognized; more than one means the catalog is ambiguous.
	Matches []catalog.Match
}

// Name returns the file's base name.
func (f File) Name() string {
	return path.Base(f.Path)
}

// Trash is a trash folder left behind by a desktop or by Windows.
type Trash struct {
	Path  string // relative to Result.Root, with / separators
	Bytes int64
}

// Problem is a file or folder that couldn't be read.
type Problem struct {
	Path string // relative to Result.Root, with / separators
	Err  error
}

func (p Problem) Error() string {
	return p.Path + ": " + p.Err.Error()
}

// Scan walks root and lists the files that match a catalog entry or have an
// extension the profile lists. It reads only the start (and sometimes the
// end) of each such file and never writes anything.
func Scan(ctx context.Context, root string, cat *catalog.Catalog, opts Options) (*Result, error) {
	profile := opts.Profile
	if profile == "" {
		profile = Ventoy
	}
	if _, err := ParseProfile(string(profile)); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", root)
	}
	var skip []string
	for _, s := range opts.Skip {
		abs, err := filepath.Abs(s)
		if err != nil {
			return nil, err
		}
		skip = append(skip, abs)
	}

	s := &scanner{ctx: ctx, root: root, cat: cat, profile: profile, skip: skip, res: &Result{Root: root, Profile: profile}}
	if err := filepath.WalkDir(root, s.visit); err != nil {
		return nil, err
	}
	slices.SortFunc(s.res.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return s.res, nil
}

type scanner struct {
	ctx     context.Context
	root    string
	cat     *catalog.Catalog
	profile Profile
	skip    []string
	res     *Result
}

func (s *scanner) visit(p string, d fs.DirEntry, err error) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if p == s.root {
		return err // an unreadable root fails the whole scan
	}
	rel := s.rel(p)
	if err != nil {
		s.problem(rel, err)
		if d != nil && d.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	if d.IsDir() {
		return s.visitDir(p, rel, d)
	}
	return s.visitFile(p, rel, d)
}

func (s *scanner) visitDir(p, rel string, d fs.DirEntry) error {
	name := d.Name()
	atRoot := !strings.Contains(rel, "/")
	switch {
	case isTrash(name):
		s.res.Trash = append(s.res.Trash, Trash{Path: rel, Bytes: s.dirSize(p)})
	case !s.profile.Recursive():
	case strings.EqualFold(name, ".isoshelf"):
	case atRoot && strings.EqualFold(name, "ventoy"):
	case strings.EqualFold(name, "System Volume Information"):
	case slices.ContainsFunc(s.skip, func(dir string) bool { return samePath(dir, p) }):
	default:
		return nil
	}
	return fs.SkipDir
}

func (s *scanner) visitFile(p, rel string, d fs.DirEntry) error {
	if !d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0 {
		return nil // devices, pipes and the like
	}
	name := d.Name()
	matches := s.cat.Match(name)
	bootable := s.profile.Lists(name)
	if len(matches) == 0 && !bootable {
		return nil
	}
	info, err := os.Stat(p) // follows symlinks
	if err != nil {
		s.problem(rel, err)
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	kind, err := sniff.File(p)
	if err != nil {
		s.problem(rel, err) // still list the file, with an unknown kind
	}
	s.res.Files = append(s.res.Files, File{
		Path:     rel,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Kind:     kind,
		Bootable: bootable,
		Matches:  matches,
	})
	return nil
}

// dirSize adds up the sizes of the files under dir, skipping anything that
// can't be read.
func (s *scanner) dirSize(dir string) int64 {
	var total int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if s.ctx.Err() != nil {
			return s.ctx.Err()
		}
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func (s *scanner) rel(p string) string {
	rel, err := filepath.Rel(s.root, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(rel)
}

func (s *scanner) problem(rel string, err error) {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err // the path is already in the Problem
	}
	s.res.Problems = append(s.res.Problems, Problem{Path: rel, Err: err})
}

// isTrash reports whether a folder is a trash folder: .Trash-<uid> from Linux
// desktops, .Trashes from macOS, or $RECYCLE.BIN from Windows.
func isTrash(name string) bool {
	return strings.HasPrefix(name, ".Trash-") || name == ".Trashes" || strings.EqualFold(name, "$RECYCLE.BIN")
}

// samePath compares two absolute paths, ignoring case on Windows.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
