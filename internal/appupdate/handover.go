package appupdate

import (
	"encoding/json"
	"strings"
)

// EnvHandover carries what one isoshelf tells the next across a restart. It
// is how the new program comes back on the same port - the page in the
// browser is at that address, and by default isoshelf picks any free port -
// and how it knows it is on trial.
const EnvHandover = "ISOSHELF_HANDOVER"

// envToken is the link's secret, which isoshelf already reads from the
// environment. Handing it over keeps the page in the browser signed in.
const envToken = "ISOSHELF_TOKEN"

// Handover is what the old program tells the new one.
type Handover struct {
	// Port is the one to listen on.
	Port int `json:"port"`
	// Stage is the staging folder of an update just put in place. The new
	// program confirms it once it is serving, or undoes it if it can't.
	Stage string `json:"stage,omitempty"`
	// From is the version that handed over.
	From string `json:"from,omitempty"`
	// Failed says why an update didn't take, when the old program is back.
	Failed string `json:"failed,omitempty"`
}

// ReadHandover returns the handover this program was started with, if any.
func ReadHandover(getenv func(string) string) (Handover, bool) {
	raw := getenv(EnvHandover)
	if raw == "" {
		return Handover{}, false
	}
	var h Handover
	if err := json.Unmarshal([]byte(raw), &h); err != nil || h.Port <= 0 {
		return Handover{}, false
	}
	return h, true
}

// Environ is env with this handover and the link's token in it, and any
// earlier ones taken out.
func (h Handover) Environ(env []string, token string) []string {
	data, _ := json.Marshal(h)
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		if strings.HasPrefix(kv, EnvHandover+"=") || strings.HasPrefix(kv, envToken+"=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, EnvHandover+"="+string(data))
	if token != "" {
		out = append(out, envToken+"="+token)
	}
	return out
}
