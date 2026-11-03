package main

import "testing"

// Trusting a proxy is opt-in, and only a plain yes counts: a value that merely
// looks like one (or a typo) leaves it off, which is the safe way to be wrong.
func TestTrustingAProxyIsOptInAndNeedsAPlainYes(t *testing.T) {
	for value, want := range map[string]bool{
		"": false, "0": false, "false": false, "no": false, "maybe": false, "tru": false,
		"1": true, "true": true, "TRUE": true, " yes ": true, "on": true,
	} {
		e := &env{getenv: func(name string) string {
			if name == trustProxyEnv {
				return value
			}
			return ""
		}}
		if got := trustProxy(e); got != want {
			t.Errorf("%s=%q: trusts the proxy = %v, want %v", trustProxyEnv, value, got, want)
		}
	}
}
