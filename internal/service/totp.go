package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP per RFC 6238 with the parameters every authenticator app assumes:
// HMAC-SHA1, 6 digits, 30-second steps. Implemented here rather than pulled
// in as a dependency — it's a few lines of stdlib.
const (
	totpDigits = 6
	totpPeriod = 30
	// totpSkew is how many steps either side of "now" are accepted, to
	// tolerate clock drift between the server and the user's phone.
	totpSkew = 1
)

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return totpEncoding.EncodeToString(raw), nil
}

// totpCode computes the code for one time step.
func totpCode(key []byte, step uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], step)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	mod := uint32(1)
	for range totpDigits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, value%mod)
}

// totpMatch checks code against the steps around now and returns the step
// it matched, so the caller can refuse to accept that step a second time.
// Steps at or below lastStep are skipped (replay protection).
func totpMatch(secret, code string, now time.Time, lastStep uint64) (step uint64, ok bool) {
	key, err := totpEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return 0, false
	}
	code = normalizeTOTPCode(code)
	if len(code) != totpDigits {
		return 0, false
	}

	current := uint64(now.Unix()) / totpPeriod
	for delta := -totpSkew; delta <= totpSkew; delta++ {
		s := current + uint64(int64(delta))
		if s <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(totpCode(key, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

// normalizeTOTPCode drops the spaces/dashes people type when copying a code
// shown as "123 456".
func normalizeTOTPCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return -1
		}
		return r
	}, code)
}

// totpURI is the otpauth:// link authenticator apps import, either pasted
// directly or (in apps that support it) opened as a link.
func totpURI(secret, username string) string {
	label := url.PathEscape("caddy-ui:" + username)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", "caddy-ui")
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}
