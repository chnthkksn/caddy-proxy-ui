package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"caddy-proxy-ui/internal/store"
)

func newTestAuth(t *testing.T) *AuthService {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	a := NewAuthService(st)
	if err := a.Setup("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	return a
}

func currentCode(t *testing.T, secret string, offset time.Duration) string {
	t.Helper()
	key, err := totpEncoding.DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return totpCode(key, uint64(time.Now().Add(offset).Unix())/totpPeriod)
}

func enableTOTP(t *testing.T, a *AuthService) string {
	t.Helper()
	secret, _, err := a.BeginTOTPSetup()
	if err != nil {
		t.Fatal(err)
	}
	// Use the previous step's code so the next "current" code is still
	// spendable afterwards.
	if err := a.EnableTOTP(currentCode(t, secret, -totpPeriod*time.Second)); err != nil {
		t.Fatal(err)
	}
	return secret
}

func TestLoginWithoutTOTPIssuesSession(t *testing.T) {
	a := newTestAuth(t)
	res, err := a.Login("admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if res.Token == "" || res.TOTPChallenge != "" {
		t.Fatalf("want token and no challenge, got %+v", res)
	}
	if !a.ValidateSession(res.Token) {
		t.Fatal("session should be valid")
	}
}

func TestTOTPSetupRequiresValidCode(t *testing.T) {
	a := newTestAuth(t)
	if _, _, err := a.BeginTOTPSetup(); err != nil {
		t.Fatal(err)
	}
	if err := a.EnableTOTP("000000"); !errors.Is(err, ErrInvalidTOTPCode) {
		t.Fatalf("want ErrInvalidTOTPCode, got %v", err)
	}
	if on, _ := a.TOTPEnabled(); on {
		t.Fatal("a failed confirmation must not enable two-factor")
	}
}

func TestLoginWithTOTP(t *testing.T) {
	a := newTestAuth(t)
	secret := enableTOTP(t, a)

	res, err := a.Login("admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if res.Token != "" || res.TOTPChallenge == "" {
		t.Fatalf("want challenge and no token, got %+v", res)
	}

	if _, err := a.CompleteTOTPLogin(res.TOTPChallenge, "000000"); !errors.Is(err, ErrInvalidTOTPCode) {
		t.Fatalf("want ErrInvalidTOTPCode, got %v", err)
	}

	code := currentCode(t, secret, 0)
	token, err := a.CompleteTOTPLogin(res.TOTPChallenge, code)
	if err != nil {
		t.Fatal(err)
	}
	if !a.ValidateSession(token) {
		t.Fatal("session should be valid")
	}

	// The challenge is single-use, and so is the code.
	if _, err := a.CompleteTOTPLogin(res.TOTPChallenge, code); !errors.Is(err, ErrTOTPChallengeInvalid) {
		t.Fatalf("want ErrTOTPChallengeInvalid for reused challenge, got %v", err)
	}
	res2, _ := a.Login("admin", "password123")
	if _, err := a.CompleteTOTPLogin(res2.TOTPChallenge, code); !errors.Is(err, ErrInvalidTOTPCode) {
		t.Fatalf("want ErrInvalidTOTPCode for replayed code, got %v", err)
	}
}

func TestTOTPChallengeAttemptCap(t *testing.T) {
	a := newTestAuth(t)
	secret := enableTOTP(t, a)
	res, _ := a.Login("admin", "password123")

	for i := 0; i < totpChallengeMaxAttempts-1; i++ {
		if _, err := a.CompleteTOTPLogin(res.TOTPChallenge, "000000"); !errors.Is(err, ErrInvalidTOTPCode) {
			t.Fatalf("attempt %d: want ErrInvalidTOTPCode, got %v", i, err)
		}
	}
	if _, err := a.CompleteTOTPLogin(res.TOTPChallenge, "000000"); !errors.Is(err, ErrTOTPChallengeInvalid) {
		t.Fatalf("want challenge burned after %d attempts, got %v", totpChallengeMaxAttempts, err)
	}
	if _, err := a.CompleteTOTPLogin(res.TOTPChallenge, currentCode(t, secret, 0)); !errors.Is(err, ErrTOTPChallengeInvalid) {
		t.Fatalf("a burned challenge must not accept even a correct code, got %v", err)
	}
}

func TestDisableTOTPAndResetPassword(t *testing.T) {
	a := newTestAuth(t)
	secret := enableTOTP(t, a)

	if err := a.DisableTOTP("wrong-password", currentCode(t, secret, 0)); !errors.Is(err, ErrCurrentPasswordIncorrect) {
		t.Fatalf("want ErrCurrentPasswordIncorrect, got %v", err)
	}
	if err := a.DisableTOTP("password123", currentCode(t, secret, 0)); err != nil {
		t.Fatal(err)
	}
	if on, _ := a.TOTPEnabled(); on {
		t.Fatal("two-factor should be off")
	}

	enableTOTP(t, a)
	if err := a.ResetPassword("newpassword123"); err != nil {
		t.Fatal(err)
	}
	if on, _ := a.TOTPEnabled(); on {
		t.Fatal("reset-password should turn two-factor off")
	}
}
