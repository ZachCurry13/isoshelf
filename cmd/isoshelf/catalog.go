package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/catupdate"
	"github.com/ZachCurry13/isoshelf/internal/usercat"
)

// loadCatalog loads the catalog named by the --catalog flag, else the user's
// own copy in the config folder, else the copy isoshelf has downloaded, else
// the built-in one. It also reports which of those it used, because isoshelf
// only ever replaces its own copy.
func loadCatalog(dirs appdir.Dirs, flagPath string) (*catalog.Catalog, string, error) {
	name := flagPath
	if name == "" {
		name = filepath.Join(dirs.Config, "catalog.toml")
		if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
			built, err := catalog.Default()
			// A downloaded catalog that no longer loads is skipped rather
			// than fatal: the built-in one always works. One downloaded
			// before this binary was built is skipped too, so upgrading
			// isoshelf never steps back to an older list of images.
			if downloaded, dlErr := catupdate.Load(dirs.Config); dlErr == nil && downloaded.Newer(built) {
				return downloaded, catupdate.SourceDownloaded, nil
			}
			return built, catupdate.SourceBuiltIn, err
		}
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return nil, "", err
	}
	cat, err := catalog.Load(os.DirFS(filepath.Dir(abs)), filepath.Base(abs))
	return cat, catupdate.SourceOwn, err
}

// withOwnImages adds the images the user named themselves. They live in their
// own file so that the catalog underneath can still update itself. A file
// that no longer loads is reported, not fatal: isoshelf carries on with the
// images it knows.
func withOwnImages(cat *catalog.Catalog, dirs appdir.Dirs, warn io.Writer) *catalog.Catalog {
	mine, err := usercat.Load(dirs.Config)
	if err == nil && mine != nil {
		var merged *catalog.Catalog
		if merged, err = catalog.Merge(cat, mine); err == nil {
			return merged
		}
	}
	if err != nil {
		fmt.Fprintf(warn, "isoshelf: %s: %v\n", usercat.Path(dirs.Config), err)
	}
	return cat
}
