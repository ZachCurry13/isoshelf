package appupdate

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// StageDir is where an update waits next to the program it replaces: the
// same folder, so putting it in place is a rename rather than a copy.
const StageDir = ".isoshelf-update"

// Staged is an update downloaded and checked, waiting to be put in place.
type Staged struct {
	// Version is the release, "v0.6.1".
	Version string
	// Dir is the folder it waits in.
	Dir   string
	Files []StagedFile
}

// StagedFile is one new program and where it goes.
type StagedFile struct {
	Target
	// New is the downloaded program.
	New string
}

// Source is where Prepare finds a release.
type Source struct {
	// API is GitHub's address for a release by tag; empty means GitHub's own.
	API string
	// Client asks GitHub and fetches the checksum file and its signature.
	Client *remote.Client
	// Fetch downloads the programs, which are too big for Client.
	Fetch *fetch.Client
	// Key checks the signature; nil means the key built into isoshelf.
	Key ed25519.PublicKey
}

// asset is one file of a release, as GitHub lists it.
type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// ErrNoKey means this build can't check an update, and so won't install one.
var ErrNoKey = errors.New("this build of isoshelf can't check an update's signature, so it can't update itself; download the new version from the releases page")

// Prepare downloads release tag for every target and checks it: the checksum
// file must carry the project's signature, and every program must match its
// line in that file, before anything is kept. Nothing next to the program is
// touched but the staging folder.
func Prepare(ctx context.Context, src Source, tag string, targets []Target, progress func(done, total int64)) (*Staged, error) {
	key := src.Key
	if key == nil {
		var ok bool
		if key, ok = releaseKey(); !ok {
			return nil, ErrNoKey
		}
	}
	api := src.API
	if api == "" {
		api = "https://api.github.com/repos/" + Repo + "/releases/tags/"
	}
	resp, err := src.Client.Get(ctx, api+tag)
	if err != nil {
		return nil, fmt.Errorf("isoshelf %s: %w", tag, err)
	}
	var rel struct {
		Assets []asset `json:"assets"`
	}
	if err := json.Unmarshal(resp.Body, &rel); err != nil {
		return nil, fmt.Errorf("isoshelf %s: %w", tag, err)
	}

	sums, err := signedSums(ctx, src.Client, key, rel.Assets, tag)
	if err != nil {
		return nil, err
	}

	var want []asset
	for _, t := range targets {
		a, ok := findAsset(rel.Assets, tag, t.Suffix)
		if !ok {
			return nil, fmt.Errorf("isoshelf %s has no program for %s", tag, t.Suffix)
		}
		want = append(want, a)
	}
	var total, done int64
	for _, a := range want {
		total += a.Size
	}

	dir := filepath.Join(filepath.Dir(targets[0].Path), StageDir)
	os.RemoveAll(dir) // isoshelf's own leftovers from an update that didn't finish
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	out := &Staged{Version: tag, Dir: dir}
	for i, a := range want {
		c, ok := verify.Strongest(sums, a.Name)
		if !ok || c.Algorithm != verify.SHA256 {
			return nil, fmt.Errorf("the signed checksum file doesn't list %s", a.Name)
		}
		before := done
		res, err := src.Fetch.Download(ctx, fetch.Request{
			URLs: []string{a.URL}, Filename: a.Name, Dir: dir, Size: a.Size, Checksum: &c, Replace: true,
		}, func(p fetch.Progress) {
			if progress != nil && p.Stage == fetch.Downloading {
				progress(before+p.Done, total)
			}
		})
		if err != nil {
			return nil, err
		}
		done += a.Size
		if err := os.Chmod(res.Path, 0o755); err != nil {
			return nil, err
		}
		out.Files = append(out.Files, StagedFile{Target: targets[i], New: res.Path})
	}
	return out, nil
}

// signedSums fetches the release's checksum file and its signature, and
// returns the checksums only if the signature is the project's.
func signedSums(ctx context.Context, client *remote.Client, key ed25519.PublicKey, assets []asset, tag string) ([]verify.Checksum, error) {
	var sums, sig *asset
	for i := range assets {
		switch assets[i].Name {
		case SumsFile:
			sums = &assets[i]
		case SumsFile + SignatureSuffix:
			sig = &assets[i]
		}
	}
	if sums == nil || sig == nil {
		return nil, fmt.Errorf("isoshelf %s isn't signed, so it won't install itself; download it from the releases page", tag)
	}
	body, err := client.Get(ctx, sums.URL)
	if err != nil {
		return nil, err
	}
	signature, err := client.Get(ctx, sig.URL)
	if err != nil {
		return nil, err
	}
	if err := Verify(key, body.Body, string(signature.Body)); err != nil {
		return nil, err
	}
	return verify.ParseManifest(body.Body), nil
}

// findAsset finds a program by the end of its name, never the whole name:
// release files carry the version, so an exact name goes stale every release.
func findAsset(assets []asset, tag, suffix string) (asset, bool) {
	for _, a := range assets {
		if strings.HasPrefix(a.Name, "isoshelf-"+tag+"-") && strings.HasSuffix(a.Name, "-"+suffix) {
			return a, true
		}
	}
	return asset{}, false
}
