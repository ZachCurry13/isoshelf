//go:build !windows

package drives

// label has nothing to add: a drive is mounted at a path that already names
// it, such as /media/you/Ventoy.
func label(string) string { return "" }
