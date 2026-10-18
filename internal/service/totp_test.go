package service

import (
	"testing"
	"time"
)

// RFC 6238 appendix B vectors for SHA1, truncated to the 6 digits we use
// (the RFC lists 8-digit codes; the last 6 are the 6-digit code).
func TestTOTPCodeRFCVectors(t *testing.T) {
	key := []byte("12345678901234567890")
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, c := range cases {
		if got := totpCode(key, uint64(c.unix)/totpPeriod); got != c.want {
			t.Errorf("t=%d: got %s, want %s", c.unix, got, c.want)
		}
	}
}

func TestTOTPMatch(t *testing.T) {
	secret := totpEncoding.EncodeToString([]byte("12345678901234567890"))
	now := time.Unix(1111111111, 0)

	step, ok := totpMatch(secret, "050 471", now, 0)
	if !ok {
		t.Fatal("expected current code to match")
	}

	// A code from one step earlier is still accepted (clock drift)...
	if _, ok := totpMatch(secret, "081804", now, 0); !ok {
		t.Error("expected previous-step code to match")
	}
	// ...but not once a step at or after it has been used.
	if _, ok := totpMatch(secret, "050471", now, step); ok {
		t.Error("expected replayed code to be rejected")
	}
	if _, ok := totpMatch(secret, "000000", now, 0); ok {
		t.Error("expected wrong code to be rejected")
	}
	if _, ok := totpMatch(secret, "12345", now, 0); ok {
		t.Error("expected short code to be rejected")
	}
}
