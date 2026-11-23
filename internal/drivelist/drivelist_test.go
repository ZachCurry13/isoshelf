package drivelist

import (
	"strings"
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	l := List{Folder: "/media/stick", Made: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Images: []Image{
		{ID: "ubuntu-desktop-lts", Name: "Ubuntu Desktop LTS", Version: "24.04.3", File: "ubuntu-24.04.3-desktop-amd64.iso", Starred: true},
		{ID: "debian-netinst-amd64", Name: "Debian netinst"},
	}}
	data, err := Encode(l)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# isoshelf drive list") {
		t.Error("the file doesn't say what it is")
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Images) != 2 || got.Images[0].ID != "debian-netinst-amd64" || !got.Images[1].Starred || got.Folder != "/media/stick" {
		t.Errorf("got %+v", got)
	}
}

func TestDecodeIsLenientButNotCareless(t *testing.T) {
	l, err := Decode([]byte("[[image]]\nid = \"a\"\nnote = \"added by hand\"\n[[image]]\nid = \"a\"\n"))
	if err != nil || len(l.Images) != 1 {
		t.Errorf("a hand-edited list: %+v, %v", l, err)
	}
	for _, bad := range []string{"", "[[image]]\nname = \"no id\"\n", "not toml at all ["} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Errorf("%q was read as a list", bad)
		}
	}
}
