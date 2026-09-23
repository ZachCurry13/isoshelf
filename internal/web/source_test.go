package web

import (
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/peer"
)

// Copying from another isoshelf is the one feature whose whole value is that
// it is faster, and the dock said "Downloading 40%" either way. These are the
// words it says instead, and the one that matters most is the negative: a
// peer that is set up but never used should read "the internet", because that
// is how somebody finds out their setting is doing nothing.
func TestWhereADownloadIsComingFrom(t *testing.T) {
	near := &peer.Client{Address: "nas.local:8765", FolderName: "the NAS"}

	for _, c := range []struct {
		why  string
		url  string
		near *peer.Client
		want string
	}{
		{"the peer, by the name it gave", "http://nas.local:8765/api/share/file?name=x.iso", near, "the NAS"},
		{"the peer, whatever case the host is written in", "http://NAS.local:8765/x.iso", near, "the NAS"},
		{"anywhere else, with a peer set up", "https://releases.ubuntu.com/x.iso", near, "the internet"},
		{"anywhere else, with no peer at all", "https://releases.ubuntu.com/x.iso", nil, "the internet"},
		{"the same host on another port is not the peer", "http://nas.local:9000/x.iso", near, "the internet"},
		{"nothing to say yet", "", near, ""},
		{"something that isn't a URL", "not a url", near, ""},
	} {
		if got := sourceName(c.url, c.near); got != c.want {
			t.Errorf("%s: sourceName(%q) = %q, want %q", c.why, c.url, got, c.want)
		}
	}

	// A peer that never said what its folder is called still has an address,
	// and an address nobody recognizes beats no answer at all.
	nameless := &peer.Client{Address: "http://192.168.1.9:8765"}
	if got, want := sourceName("http://192.168.1.9:8765/x.iso", nameless), "192.168.1.9:8765"; got != want {
		t.Errorf("a peer with no name is %q, want %q", got, want)
	}
}

// And the word reaches the page: the running download carries it, and so does
// the record of it once it has finished.
func TestThePageIsToldWhereADownloadCameFrom(t *testing.T) {
	s := serverMode(t, false)

	s.mu.Lock()
	s.downloading = &run{kind: "update", job: &job{id: 1, name: "Ubuntu"}, from: "the NAS"}
	current := s.downloadsLocked().Current
	s.mu.Unlock()
	if current == nil || current.From != "the NAS" {
		t.Errorf("the running download tells the page %+v, want From \"the NAS\"", current)
	}

	s.mu.Lock()
	s.downloading = nil
	s.finished = []finishedJob{{job: job{id: 1, name: "Ubuntu"}, outcome: "done", from: "the NAS"}}
	finished := s.downloadsLocked().Finished
	s.mu.Unlock()
	if len(finished) != 1 || finished[0].From != "the NAS" {
		t.Errorf("the finished download tells the page %+v, want From \"the NAS\"", finished)
	}
}
