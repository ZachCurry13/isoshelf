package state

import (
	"maps"
	"slices"
)

// UsualSet returns the ids of the entries normally kept on this target: the
// starred ones, plus those seen in at least 2 of the last 10 scans, less any
// somebody has said to stop expecting.
func (s *State) UsualSet() []string {
	return usualSet(s.Tracks, s.History)
}

// Missing returns the usual-set entries the latest scan didn't find.
func (s *State) Missing() []string {
	var last []string
	if len(s.History) > 0 {
		last = s.History[len(s.History)-1].Entries
	}
	var missing []string
	for _, id := range s.UsualSet() {
		if !slices.Contains(last, id) {
			missing = append(missing, id)
		}
	}
	return missing
}

func usualSet(tracks map[string]Track, history []ScanRecord) []string {
	seen := map[string]int{}
	for _, rec := range history[max(0, len(history)-usualWindow):] {
		for _, id := range rec.Entries {
			seen[id]++
		}
	}
	usual := map[string]bool{}
	for id, n := range seen {
		if n >= usualMinSeen {
			usual[id] = true
		}
	}
	for id, t := range tracks {
		if t.Starred {
			usual[id] = true
		}
		if t.NotExpected {
			delete(usual, id)
		}
	}
	return slices.Sorted(maps.Keys(usual))
}
