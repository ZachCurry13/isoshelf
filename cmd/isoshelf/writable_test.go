package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A folder isoshelf can write to says nothing at all. Most starts are this
// one, and a warning nobody needs is a warning everybody learns to skip.
func TestWritableFoldersSayNothing(t *testing.T) {
	var out bytes.Buffer
	checkWritable(&out, filepath.Join(t.TempDir(), "config"), t.TempDir())
	if out.Len() > 0 {
		t.Errorf("a start with nothing wrong printed:\n%s", out.String())
	}
	// And the check left nothing behind in either folder.
	dir := t.TempDir()
	checkWritable(&out, dir, dir)
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("the check left %d files behind: %v", len(names), names)
	}
}

// A folder it can't write to says which folder, what it costs, and what to
// do about it - including which user it is running as, which is the half
// nobody can work out from outside the container.
func TestUnwritableFoldersSayHowToFixIt(t *testing.T) {
	// A regular file where a folder should be: no user can make a folder
	// inside that one, root included, so this behaves the same everywhere
	// the tests run.
	parent := t.TempDir()
	blocked := filepath.Join(parent, "not-a-folder")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(blocked, "isoshelf")

	var out bytes.Buffer
	checkWritable(&out, config, "")
	got := out.String()
	for _, want := range []string{
		"can't write to " + config,
		"the link's secret changes every restart",
		"To fix it, " + parent,
		"TrueNAS",
		"on the host machine",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the complaint doesn't mention %q:\n%s", want, got)
		}
	}
	// It names the nearest folder that really exists, which is the one whose
	// permissions are in the way - not the one isoshelf wanted to make inside
	// it, which nobody can go and look at.
	if !strings.Contains(got, "To fix it, "+parent+" has to belong") {
		t.Errorf("it points at a folder that isn't there:\n%s", got)
	}
	// On Unix it says which user, because that is what has to match.
	if runtime.GOOS != "windows" && !strings.Contains(got, "user ") {
		t.Errorf("it doesn't say which user isoshelf runs as:\n%s", got)
	}
}

// The images folder is a different warning: isoshelf keeps working, and says
// what it can and can't do, rather than implying it has stopped.
func TestAnUnwritableImagesFolderSaysWhatStillWorks(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "images")
	if err := os.Mkdir(blocked, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := canWrite(blocked); err == nil {
		// Running as root, or on a system where the mode doesn't bite. The
		// warning's wording is what this test is about, so check it directly.
		t.Skip("this user can write to a 0555 folder, so there is nothing to warn about")
	}
	var out bytes.Buffer
	checkWritable(&out, "", blocked)
	got := out.String()
	for _, want := range []string{"still list what's in it", "download anything into it", "To fix it, " + blocked} {
		if !strings.Contains(got, want) {
			t.Errorf("the complaint doesn't mention %q:\n%s", want, got)
		}
	}
}

// A folder that isn't there at all is somebody else's complaint - the mount
// check has already said so, and saying it twice in different words reads
// like two different problems.
func TestAMissingImagesFolderIsNotWarnedAboutTwice(t *testing.T) {
	var out bytes.Buffer
	checkWritable(&out, "", filepath.Join(t.TempDir(), "nothing-mounted-here"))
	if out.Len() > 0 {
		t.Errorf("a missing folder was complained about again:\n%s", out.String())
	}
}

// isoshelf's own folder inside the images folder is checked separately,
// because it is often older than the current arrangement: made on an earlier
// run by whichever user isoshelf was then. The folder around it can be
// perfectly writable while that one isn't, and every download is staged
// there - which is how a folder that looks fine fails at the first download
// with nothing having warned about it.
func TestTheFolderInsideIsCheckedToo(t *testing.T) {
	images := t.TempDir()
	inside := filepath.Join(images, ".isoshelf")
	if err := os.Mkdir(inside, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := canWrite(inside); err == nil {
		t.Skip("this user can write to a 0555 folder, so there is nothing to warn about")
	}

	var out bytes.Buffer
	checkWritable(&out, "", images)
	got := out.String()
	if got == "" {
		t.Fatal("a folder whose .isoshelf can't be written to said nothing at all")
	}
	for _, want := range []string{
		inside,
		"downloads are",
		"permission denied",
		"give that one folder to the user isoshelf runs as",
		"chown -R",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the complaint doesn't mention %q:\n%s", want, got)
		}
	}
	// The advice for this case is the opposite of the advice for the mount
	// itself, and saying the wrong one would break the folder that works.
	if !strings.Contains(got, "Do NOT") {
		t.Errorf("it doesn't warn against changing the app's user instead:\n%s", got)
	}
}
