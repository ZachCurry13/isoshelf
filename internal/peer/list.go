package peer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// SharedFile is one file the other isoshelf offers, with what it says about
// it. That is its word, not proof: see List.
type SharedFile struct {
	Name     string       `json:"name"`
	SHA256   string       `json:"sha256"`
	Size     int64        `json:"size"`
	Entry    string       `json:"entry,omitempty"`
	Version  string       `json:"version,omitempty"`
	Assigned bool         `json:"assigned,omitempty"`
	Origin   state.Origin `json:"origin,omitzero"`
	Since    time.Time    `json:"since,omitzero"`
}

// List asks what the other isoshelf would hand over (v0.8.1). What it says
// about each file - what it is, where it came from - is recorded as its
// account, never as a check: the only thing a copy's hash can prove is that
// it arrived as it is over there.
func (c *Client) List(ctx context.Context) ([]SharedFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/api/share/list", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, c.cantReach(err)
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden:
		return nil, fmt.Errorf("the isoshelf at %s isn't sharing its images; turn sharing on in its Settings", c.Address)
	default:
		return nil, fmt.Errorf("the other isoshelf answered %s", res.Status)
	}
	var answer struct {
		Files []SharedFile `json:"files"`
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		return nil, fmt.Errorf("the other isoshelf gave an answer isoshelf couldn't read: %w", err)
	}
	return answer.Files, nil
}
