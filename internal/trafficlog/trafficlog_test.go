package trafficlog

import "testing"

func TestHostWithoutPort(t *testing.T) {
	cases := map[string]string{
		"app.example.com":       "app.example.com", // the common case: no port
		"app.example.com:18443": "app.example.com", // non-standard port
		"app.example.com:443":   "app.example.com",
		"":                      "",
		"127.0.0.1:8080":        "127.0.0.1",
		"[::1]:8080":            "::1",
		"[::1]":                 "::1", // bracketed literal, no port
	}
	for in, want := range cases {
		if got := hostWithoutPort(in); got != want {
			t.Errorf("hostWithoutPort(%q) = %q, want %q", in, got, want)
		}
	}
}
