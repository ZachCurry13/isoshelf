package settings

import "testing"

// The download limit is megabits a second, kept between none and a sane top.
func TestDownloadLimitIsCleanedAndConverted(t *testing.T) {
	for ask, want := range map[int]int{-3: 0, 0: 0, 200: 200, MaxDownloadLimit + 1: MaxDownloadLimit} {
		if got := CleanDownloadLimit(ask); got != want {
			t.Errorf("CleanDownloadLimit(%d) = %d, want %d", ask, got, want)
		}
	}
	if got := BytesPerSecond(200); got != 25_000_000 {
		t.Errorf("200 megabits is %d bytes a second, want 25,000,000", got)
	}
	if got := BytesPerSecond(-1); got != 0 {
		t.Errorf("a negative limit is %d, want none", got)
	}
}
