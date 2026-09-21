// Package lastcheck remembers what each image's project said the last time
// isoshelf asked. Opening the page then costs nothing: an answer from within
// the last day is used as it stands, and only the images nobody has asked
// about lately are looked up again.
//
// Only what the check needs to work out a status is kept, and none of it is
// trusted for a download: a download asks the project again from scratch, so
// a remembered answer can never put the wrong file on a drive.
package lastcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/source"
)

// FileName is where the answers are kept inside the config folder.
const FileName = "last-check.json"

// Fresh is how long an answer is used before the project is asked again. A
// day is the maintainer's decision: distributions don't release hourly, and
// someone who wants to know right now presses Refresh.
const Fresh = 24 * time.Hour

// forgetAfter is when an answer stops being worth keeping at all. Without it
// the file would grow by every image that ever left the catalog.
const forgetAfter = 30 * 24 * time.Hour

// fileVersion is the format of the file on disk. A file from the future, or
// one that can't be read, is treated as no file: these are answers isoshelf
// can always ask for again.
const fileVersion = 1

// Answer is what one image's project said, and when it said it.
type Answer struct {
	At       time.Time         `json:"at"`
	Release  *source.Release   `json:"release,omitempty"`
	Artifact *resolve.Artifact `json:"artifact,omitempty"`
}

// Answers is the whole file: one answer per catalog entry.
type Answers struct {
	Entries map[string]Answer

	// Now defaults to time.Now, Fresh to the constant above.
	Now   func() time.Time
	Fresh time.Duration

	// mu guards Entries, because a check asks about several images at once.
	mu      sync.Mutex
	dir     string
	changed bool
}

// onDisk is the file's shape. It is kept apart from Answers so the mutex and
// the test hooks never end up in the file.
type onDisk struct {
	Version int               `json:"version"`
	Entries map[string]Answer `json:"entries,omitempty"`
}

// Load reads the remembered answers, or returns an empty set when there are
// none to read.
func Load(configDir string) *Answers {
	a := &Answers{Entries: map[string]Answer{}, dir: configDir}
	if configDir == "" {
		return a
	}
	data, err := os.ReadFile(filepath.Join(configDir, FileName))
	if err != nil {
		return a
	}
	var read onDisk
	if json.Unmarshal(data, &read) != nil || read.Version != fileVersion {
		return a
	}
	if read.Entries != nil {
		a.Entries = read.Entries
	}
	return a
}

func (a *Answers) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *Answers) fresh() time.Duration {
	if a.Fresh > 0 {
		return a.Fresh
	}
	return Fresh
}

// Recall returns the answer for an entry when there is one and it is still
// fresh. The last result says whether there was one.
func (a *Answers) Recall(entry string) (*source.Release, *resolve.Artifact, bool) {
	if a == nil {
		return nil, nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	answer, ok := a.Entries[entry]
	if !ok || answer.Release == nil || a.now().Sub(answer.At) > a.fresh() {
		return nil, nil, false
	}
	return answer.Release, answer.Artifact, true
}

// Remember keeps what a project just said. A failure is never remembered, so
// a site that was down is asked again rather than looking like bad news for
// the rest of the day.
func (a *Answers) Remember(entry string, rel *source.Release, art *resolve.Artifact) {
	if a == nil || rel == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Entries[entry] = Answer{At: a.now(), Release: rel, Artifact: art}
	a.changed = true
}

// Asked returns when an entry's answer is from, or the zero time when there
// is nothing remembered about it.
func (a *Answers) Asked(entry string) time.Time {
	if a == nil {
		return time.Time{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Entries[entry].At
}

// Asking is a view of the answers that keeps what comes back but never
// reuses what is already there. It is what Refresh and the command line's
// own check want: there the whole point is to ask.
type Asking struct{ *Answers }

// Asking returns that view.
func (a *Answers) Asking() Asking { return Asking{a} }

// Recall always says it knows nothing, so every entry is asked about.
func (Asking) Recall(string) (*source.Release, *resolve.Artifact, bool) {
	return nil, nil, false
}

// Save writes the answers, dropping the ones too old to be worth keeping.
// Failing to save is not worth stopping for: the worst that happens is
// asking again next time.
func (a *Answers) Save() error {
	if a == nil || a.dir == "" {
		return nil
	}
	a.mu.Lock()
	if !a.changed {
		a.mu.Unlock()
		return nil
	}
	cutoff := a.now().Add(-forgetAfter)
	for id, answer := range a.Entries {
		if answer.At.Before(cutoff) {
			delete(a.Entries, id)
		}
	}
	a.changed = false
	data, err := json.MarshalIndent(onDisk{fileVersion, a.Entries}, "", "  ")
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(a.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.dir, FileName), data, 0o644)
}
