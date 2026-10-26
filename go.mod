module github.com/ZachCurry13/isoshelf

// The 1.27 line rather than a patch: the release build and the container
// take the newest 1.27 patch, so a Go security fix reaches the next release
// without anyone raising this. The weekly audit says when the released
// program needs one.
go 1.27

require github.com/pelletier/go-toml/v2 v2.4.3
