// Command mkdrive writes the sample drive's filenames into a folder, for
// looking at the page with a full list. Not part of the app.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
)

func main() {
	dir := os.Args[1]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	n := 0
	for _, list := range [][]sampledrive.File{sampledrive.Files, sampledrive.ProxmoxFolder} {
		for _, f := range list {
			if err := os.WriteFile(filepath.Join(dir, f.Name), []byte("stand-in"), 0o644); err != nil {
				panic(err)
			}
			n++
		}
	}
	fmt.Println(n, "files in", dir)
}
