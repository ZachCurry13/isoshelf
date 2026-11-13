package update

import (
	"context"
	"errors"
	"fmt"

	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/source"
)

// Installing an older version (#2): for when a new release breaks something
// and the answer is to go back to the one before. The version comes from the
// list the project still publishes, its file and checksum are found exactly
// as the newest one's are, and placing it puts the track on hold so isoshelf
// stops offering the release that broke.

// ErrNotPublished means the project no longer publishes the version asked
// for, or doesn't list it where isoshelf looks.
var ErrNotPublished = errors.New("the project no longer publishes that version")

// ErrOlderUnchecked means an older version was asked for whose download the
// project publishes no strong checksum for. The newest version may still
// download unchecked and wait for a look before it replaces anything; an
// older one is a deliberate step back to something known, and is refused.
var ErrOlderUnchecked = errors.New("the project publishes no checksum for that version")

// release is the release to install: the newest, or the one asked for.
func release(ctx context.Context, opts Options) (*source.Release, error) {
	if opts.Version == "" {
		return source.Latest(ctx, opts.Client, opts.Entry)
	}
	all, err := source.Versions(ctx, opts.Client, opts.Entry)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Version == opts.Version {
			return &all[i], nil
		}
	}
	return nil, said{ErrNotPublished, fmt.Sprintf("%s %s isn't published by the project any more, so isoshelf can't download it. Choose another version.", opts.Entry.Name, opts.Version)}
}

// said is one of the errors above in words for the page, which shows a
// failed download's error as it is.
type said struct {
	kind  error
	words string
}

func (e said) Error() string { return e.words }
func (e said) Unwrap() error { return e.kind }

// checkedEnough refuses an older version that can't be checked.
func checkedEnough(opts Options, a *resolve.Artifact) error {
	if opts.Version != "" && (a.Checksum == nil || a.Checksum.Algorithm.Weak()) && a.ImageSignature == nil {
		return said{ErrOlderUnchecked, fmt.Sprintf("The project publishes no checksum for %s %s, so isoshelf won't download it: an older version is only installed when it can be proven to be the project's own. Nothing was downloaded.", opts.Entry.Name, opts.Version)}
	}
	return nil
}

// hold puts the track on hold at the version just placed when it was chosen,
// and releases a hold when the newest one was: updating by hand to the newest
// is as clear a "the problem's fixed" as pressing Release.
func hold(opts Options) {
	t := opts.State.Track(opts.Entry.ID)
	if t.Hold == opts.Version {
		return
	}
	t.Hold = opts.Version
	opts.State.SetTrack(opts.Entry.ID, t)
}
