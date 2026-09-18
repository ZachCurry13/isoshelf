// Package catupdate keeps the catalog current without a new isoshelf
// release. The catalog is data: which images exist, where their versions are
// published, and where their checksums live. New images shouldn't have to
// wait for a new binary.
//
// Only one address is ever fetched, and only over HTTPS: the same catalog
// file isoshelf ships, from the project's own repository. A downloaded
// catalog has to parse and pass every validation rule before it replaces the
// last one, and the user's own catalog always wins over it.
package catupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
)

// Sources say where the catalog in use came from.
const (
	SourceBuiltIn    = "built-in"
	SourceDownloaded = "downloaded"
	SourceOwn        = "yours"
)

const (
	// SourceURL is the published catalog. Nothing else is ever fetched.
	SourceURL = "https://raw.githubusercontent.com/" + appupdate.Repo + "/main/internal/catalog/default.toml"

	// FileName is the downloaded copy in the config folder. isoshelf owns
	// this file and overwrites it; the user's own catalog is catalog.toml.
	FileName = "catalog-published.toml"

	stateFile     = "catalog-update.json"
	checkInterval = 24 * time.Hour
)

// Result says what a refresh did.
type Result struct {
	// Changed is true when a new catalog was saved.
	Changed bool
	// Entries is how many images the catalog holds now.
	Entries int
	// Added and Removed name the images that came and went.
	Added, Removed []string
	// CheckedAt is when the project was last asked, new catalog or not.
	CheckedAt time.Time
}

// Summary is one line for the page, or "" when nothing changed.
func (r *Result) Summary() string {
	if r == nil || !r.Changed {
		return ""
	}
	switch {
	case len(r.Added) > 0 && len(r.Removed) > 0:
		return fmt.Sprintf("Catalog updated: %s added, %s removed.", list(r.Added), list(r.Removed))
	case len(r.Added) > 0:
		return fmt.Sprintf("Catalog updated: %s added.", list(r.Added))
	case len(r.Removed) > 0:
		return fmt.Sprintf("Catalog updated: %s removed.", list(r.Removed))
	}
	return "Catalog updated: some images changed."
}

// list names up to three images, then counts the rest.
func list(names []string) string {
	switch n := len(names); {
	case n == 1:
		return names[0]
	case n <= 3:
		return strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	default:
		return fmt.Sprintf("%s and %d more", strings.Join(names[:3], ", "), n-3)
	}
}

type record struct {
	CheckedAt time.Time `json:"checked_at"`
	// Source is the address the copy came from, so changing it re-downloads.
	Source string `json:"source"`
}

// Due reports whether it is time to ask again.
func Due(configDir string, now time.Time) bool {
	rec := loadRecord(configDir)
	age := now.Sub(rec.CheckedAt)
	return rec.Source != SourceURL || rec.CheckedAt.IsZero() || age >= checkInterval || age < 0
}

// Refresh downloads the published catalog and keeps it only if it parses and
// passes validation. Anything that goes wrong leaves the copy already there
// untouched, and is reported as an error. Refresh returns a nil Result when
// it wasn't time to check yet and force is false.
func Refresh(ctx context.Context, client *remote.Client, configDir string, now time.Time, force bool) (*Result, error) {
	if configDir == "" {
		return nil, errors.New("no config folder to keep the catalog in")
	}
	if !force && !Due(configDir, now) {
		return nil, nil
	}
	resp, err := client.Get(ctx, SourceURL)
	if err != nil {
		return nil, plain(err)
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, err
	}

	// Write it under a temporary name first, so a catalog that turns out to
	// be broken never becomes the one isoshelf loads.
	tmp, err := os.CreateTemp(configDir, FileName+".*.tmp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(resp.Body); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	fetched, err := catalog.Load(os.DirFS(configDir), filepath.Base(tmp.Name()))
	if err != nil {
		return nil, fmt.Errorf("the published catalog was refused, so the one you have is kept: %w", err)
	}

	result := &Result{Entries: len(fetched.Entries), CheckedAt: now.UTC()}
	if before, err := Load(configDir); err == nil && before != nil {
		result.Added, result.Removed = difference(before, fetched)
	} else if before, err := catalog.Default(); err == nil {
		result.Added, result.Removed = difference(before, fetched)
	}
	same, err := sameFile(filepath.Join(configDir, FileName), resp.Body)
	if err != nil {
		return nil, err
	}
	if !same {
		if err := os.Rename(tmp.Name(), filepath.Join(configDir, FileName)); err != nil {
			return nil, err
		}
		result.Changed = true
	}
	saveRecord(configDir, record{CheckedAt: result.CheckedAt, Source: SourceURL})
	return result, nil
}

// Load returns the downloaded catalog, or nil when there isn't one. A copy
// that no longer loads is reported as an error so the caller can fall back to
// the built-in catalog.
func Load(configDir string) (*catalog.Catalog, error) {
	if configDir == "" {
		return nil, nil
	}
	name := filepath.Join(configDir, FileName)
	if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return catalog.Load(os.DirFS(configDir), FileName)
}

// Downloaded returns when the copy in configDir was written, or the zero time.
func Downloaded(configDir string) time.Time {
	info, err := os.Stat(filepath.Join(configDir, FileName))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// difference lists the entry names added and removed between two catalogs.
func difference(before, after *catalog.Catalog) (added, removed []string) {
	was := map[string]string{}
	for i := range before.Entries {
		was[before.Entries[i].ID] = before.Entries[i].Name
	}
	now := map[string]bool{}
	for i := range after.Entries {
		e := &after.Entries[i]
		now[e.ID] = true
		if _, had := was[e.ID]; !had {
			added = append(added, e.Name)
		}
	}
	for id, name := range was {
		if !now[id] {
			removed = append(removed, name)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	return added, removed
}

// sameFile reports whether the file already holds exactly these bytes.
func sameFile(name string, body []byte) (bool, error) {
	have, err := os.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return string(have) == string(body), nil
}

func loadRecord(configDir string) record {
	var rec record
	if data, err := os.ReadFile(filepath.Join(configDir, stateFile)); err == nil {
		json.Unmarshal(data, &rec) // a damaged record just means asking again
	}
	return rec
}

func saveRecord(configDir string, rec record) {
	if data, err := json.MarshalIndent(rec, "", "  "); err == nil {
		os.WriteFile(filepath.Join(configDir, stateFile), data, 0o644) // best effort
	}
}

// plain turns a fetch failure into something worth showing a person. The
// catalog already on hand keeps working either way, so none of these are
// alarming.
func plain(err error) error {
	var status *remote.StatusError
	if errors.As(err, &status) {
		switch {
		case status.Code == 404:
			return errors.New("the project hasn't published a list of images yet, so isoshelf is using the one it was built with")
		case status.Code >= 500:
			return errors.New("the project's site is having trouble, so the list of images isn't updated right now")
		default:
			return fmt.Errorf("the list of images couldn't be fetched (%s), so the one you have is kept", status.Status)
		}
	}
	return errors.New("couldn't reach the project to check for new images, so the list you have is kept")
}
