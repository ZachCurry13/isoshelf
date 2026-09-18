package space

import (
	"os"
	"testing"
)

// The numbers have to come from the real filesystem, so this checks they are
// sane rather than exact.
func TestOf(t *testing.T) {
	dir := t.TempDir()
	usage, err := Of(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !usage.Known() {
		t.Fatalf("no numbers for %s: %+v", dir, usage)
	}
	if usage.Free <= 0 || usage.Free > usage.Total {
		t.Errorf("free %d of total %d makes no sense", usage.Free, usage.Total)
	}
}

func TestOfMissingFolder(t *testing.T) {
	if _, err := Of(filepathJoin(t.TempDir(), "not-here")); err == nil {
		t.Error("a folder that isn't there reported space anyway")
	}
}

func TestFits(t *testing.T) {
	// A 100 GB disk keeps 1 GB spare.
	const gigabyte = 1 << 30
	usage := Usage{Free: 10 * gigabyte, Total: 100 * gigabyte}
	tests := []struct {
		size int64
		want bool
	}{
		{0, true},               // an unknown size can't be refused
		{gigabyte, true},        // plenty of room
		{8 * gigabyte, true},    // 2 GB would be left, over the 1 GB spare
		{9*gigabyte + 1, false}, // would leave less than the spare
		{20 * gigabyte, false},  // more than is there
	}
	for _, tt := range tests {
		if got := usage.Fits(tt.size); got != tt.want {
			t.Errorf("Fits(%d) = %v, want %v", tt.size, got, tt.want)
		}
	}

	// A filesystem that doesn't say never blocks a download.
	if !(Usage{}).Fits(100 * gigabyte) {
		t.Error("an unknown disk refused a download")
	}
	// A small disk keeps 1% spare rather than a whole gigabyte.
	small := Usage{Free: 500 << 20, Total: 8 << 30}
	if !small.Fits(400 << 20) {
		t.Error("400 MB should fit on a small disk with 500 MB free")
	}
}

func filepathJoin(dir, name string) string {
	return dir + string(os.PathSeparator) + name
}
