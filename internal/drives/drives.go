// Package drives asks the operating system what a drive is called, so the
// folder chooser can say "E: Ventoy" instead of leaving someone to work out
// which letter they just plugged in.
//
// Only Windows needs this: elsewhere a drive is already mounted somewhere
// named, like /media/you/Ventoy.
package drives

// Label returns the name the operating system gives the drive at path, or ""
// when it has none, can't be read, or the system doesn't name drives.
func Label(path string) string { return label(path) }
