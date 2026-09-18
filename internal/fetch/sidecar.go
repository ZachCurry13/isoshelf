package fetch

import (
	"encoding"
	"encoding/json"
	"hash"
	"os"
	"time"
)

// sidecar remembers an unfinished download next to its .part file.
type sidecar struct {
	URL      string    `json:"url"`
	ETag     string    `json:"etag,omitempty"`
	Offset   int64     `json:"offset"`
	Size     int64     `json:"size,omitempty"`
	HashSate []byte    `json:"sha256_state,omitempty"`
	Updated  time.Time `json:"updated"`
}

func readSidecar(part string) *sidecar {
	data, err := os.ReadFile(part + ".json")
	if err != nil {
		return nil
	}
	var s sidecar
	if json.Unmarshal(data, &s) != nil {
		return nil
	}
	return &s
}

func saveSidecar(part string, s *sidecar, offset int64, digest hash.Hash) {
	s.Offset, s.Updated = offset, time.Now().UTC()
	if m, ok := digest.(encoding.BinaryMarshaler); ok {
		if data, err := m.MarshalBinary(); err == nil {
			s.HashSate = data
		}
	}
	if data, err := json.Marshal(s); err == nil {
		os.WriteFile(part+".json", data, 0o644) // best effort: at worst we start over
	}
}
