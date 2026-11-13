module github.com/ZachCurry13/isoshelf

// The 1.27 line rather than a patch: the release build and the container
// take the newest 1.27 patch, so a Go security fix reaches the next release
// without anyone raising this. The weekly audit says when the released
// program needs one.
go 1.27

require (
	github.com/ProtonMail/go-crypto v1.5.2
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/ulikunitz/xz v0.5.17
)

require (
	github.com/cloudflare/circl v1.6.5 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
)
