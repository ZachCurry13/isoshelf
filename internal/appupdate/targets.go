package appupdate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Platforms are the programs a release carries, by the end of their names:
// isoshelf-v0.6.0-windows-amd64.exe in a release, isoshelf-windows-amd64.exe
// in the portable folder.
var Platforms = []string{"windows-amd64.exe", "linux-amd64", "linux-arm64"}

// Target is one program file an update replaces.
type Target struct {
	// Path is the program as it is now.
	Path string
	// To is where the new one goes. It is Path, unless Path carries a version
	// - the name a single download arrives with - in which case it is the
	// plain name, so the name never says one version while running another.
	// The maintainer's choice, 2026-09-23: a shortcut to the old name breaks
	// once, rather than the name being wrong from then on.
	To string
	// Suffix is how a release names this program: "windows-amd64.exe".
	Suffix string
	// Running is set on the one this process is.
	Running bool
}

// Suffix is the release name for a platform, or "" where isoshelf publishes
// no program.
func Suffix(goos, goarch string) string {
	s := goos + "-" + goarch
	if goos == "windows" {
		s += ".exe"
	}
	for _, p := range Platforms {
		if p == s {
			return s
		}
	}
	return ""
}

// ErrNoProgram means isoshelf publishes nothing for this system.
var ErrNoProgram = errors.New("isoshelf publishes no program for this system, so it can't update itself here")

// Targets lists what an update replaces, given the running program. In
// portable mode that is every program in the portable folder - the Windows
// one and both Linux ones - so a stick that moves between computers never
// carries two versions (the maintainer's choice, 2026-09-23). Otherwise it is
// only the program that is running.
func Targets(exe string, portable bool) ([]Target, error) {
	own := Suffix(runtime.GOOS, runtime.GOARCH)
	if own == "" {
		return nil, ErrNoProgram
	}
	// On Linux the program may have been started through a link; the file
	// to replace is the one the link points at.
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	exe, err := filepath.Abs(exe)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(exe)
	running := Target{Path: exe, To: filepath.Join(dir, plainName(filepath.Base(exe), own)), Suffix: own, Running: true}
	if !portable {
		return []Target{running}, nil
	}

	out := []Target{running}
	for _, suffix := range Platforms {
		path := filepath.Join(dir, "isoshelf-"+suffix)
		if suffix == own || sameFile(path, exe) {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			out = append(out, Target{Path: path, To: path, Suffix: suffix})
		}
	}
	return out, nil
}

// versioned is the front of a single download's name, isoshelf-v0.5.3 or
// isoshelf-v1.0.0-rc1, once the platform is taken off the end. Matched in
// that order because a version's own suffix and a platform both start with a
// hyphen: read from the front, "-windows" looked like part of the version.
var versioned = regexp.MustCompile(`^isoshelf-v\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?$`)

// plainName is the name a program keeps after an update: the plain one, when
// it arrived with a version in its name, and otherwise whatever it is called
// now - somebody who named it isoshelf.exe meant it.
func plainName(base, suffix string) string {
	if front, ok := strings.CutSuffix(base, "-"+suffix); ok && versioned.MatchString(front) {
		return "isoshelf-" + suffix
	}
	return base
}

func sameFile(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// Writable says whether isoshelf can put a new program in dir, by trying.
func Writable(dir string) error {
	f, err := os.CreateTemp(dir, ".isoshelf-write-test-*")
	if err != nil {
		return fmt.Errorf("isoshelf's folder %s can't be written to", dir)
	}
	name := f.Name()
	f.Close()
	os.Remove(name) // its own test file, made a moment ago
	return nil
}

// backupName is where the program an update replaced waits until the new one
// has started, so a new version that won't start can be undone.
func backupName(path string) string {
	return path + ".old"
}

// isBackup says whether a name is one backupName made.
func isBackup(name string) bool {
	return strings.HasPrefix(filepath.Base(name), "isoshelf") && strings.HasSuffix(name, ".old")
}
