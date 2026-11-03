package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// A project that publishes many test builds for every stable release (Bazzite
// does) pushes the stable one out of the newest ten. The recording has to keep
// it, or the tests replay a list the live check never saw.
func TestReleasesKeepsAStableOneBuriedUnderTestBuilds(t *testing.T) {
	var all []map[string]any
	for i := 0; i < 30; i++ {
		tag, pre := fmt.Sprintf("testing-%d", 30-i), true
		if i == 20 || i == 25 {
			tag, pre = fmt.Sprintf("44.2026%04d", i), false
		}
		all = append(all, map[string]any{"tag_name": tag, "prerelease": pre, "assets": []any{}})
	}
	body, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	out, err := releases(body, 10)
	if err != nil {
		t.Fatal(err)
	}
	var kept []map[string]any
	if err := json.Unmarshal(out, &kept); err != nil {
		t.Fatal(err)
	}
	if len(kept) != 12 {
		t.Fatalf("kept %d releases, want the first 10 and the 2 stable ones", len(kept))
	}
	if kept[0]["tag_name"] != "testing-30" || kept[10]["tag_name"] != "44.20260020" || kept[11]["tag_name"] != "44.20260025" {
		t.Errorf("kept the wrong ones, in the wrong order: %v ... %v, %v", kept[0]["tag_name"], kept[10]["tag_name"], kept[11]["tag_name"])
	}
}

// Fewer releases than asked for are all kept, as before.
func TestReleasesKeepsAShortListWhole(t *testing.T) {
	out, err := releases([]byte(`[{"tag_name":"v1","prerelease":false},{"tag_name":"v2","prerelease":false}]`), 10)
	if err != nil {
		t.Fatal(err)
	}
	var kept []map[string]any
	if err := json.Unmarshal(out, &kept); err != nil || len(kept) != 2 {
		t.Fatalf("kept %d (%v), want 2", len(kept), err)
	}
}
