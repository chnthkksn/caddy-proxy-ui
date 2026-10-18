package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"caddy-proxy-ui/internal/store"
)

var ErrSetupAlreadyDone = errors.New("admin account already exists")
var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrNoAdminAccount = errors.New("no administrator account exists yet — complete first-run setup via the web UI first")
var ErrCurrentPasswordIncorrect = errors.New("current password is incorrect")
var ErrInvalidTOTPCode = errors.New("invalid authentication code")
var ErrTOTPChallengeInvalid = errors.New("sign-in expired — enter your password again")
var ErrTOTPAlreadyEnabled = errors.New("two-factor authentication is already enabled")
var ErrTOTPNotEnabled = errors.New("two-factor authentication is not enabled")
var ErrTOTPSetupNotStarted = errors.New("two-factor setup was not started")
var ErrTOTPLocked = errors.New("too many wrong authentication codes — try again later")

const sessionTTL = 30 * 24 * time.Hour

// A password-verified login waiting on its TOTP code. Short-lived and
// capped on attempts, so the 6-digit code can't be brute-forced without
// also re-entering the password every few guesses.
const (
	totpChallengeTTL         = 5 * time.Minute
	totpChallengeMaxAttempts = 5
)

// The per-challenge cap alone can be sidestepped by whoever has the
// password — just log in again for a fresh challenge — so wrong codes are
// also counted for the account as a whole, across every challenge.
const (
	totpFailureWindow = 15 * time.Minute
	totpMaxFailures   = 10
)

const (
	settingTOTPSecret        = "admin_totp_secret"
	settingTOTPLastStep      = "admin_totp_last_step"
	settingTOTPPendingSecret = "admin_totp_pending_secret"
)

type totpChallenge struct {
	expiresAt time.Time
	attempts  int
}

// LoginResult is what a correct password gets you: either a session token
// straight away, or — with two-factor on — a challenge to exchange for one
// via CompleteTOTPLogin.
type LoginResult struct {
	Token         string
	TOTPChallenge string
}

// AuthService owns the single-admin account and its sessions. There is no
// default account and no default password — Setup must run once before
// anything else in the API is usable.
type AuthService struct {
	store *store.Store

	// mu guards challenges, and also serializes TOTP verification so two
	// concurrent requests can't both spend the same code.
	mu           sync.Mutex
	challenges   map[string]*totpChallenge // keyed by hashToken(challenge)
	totpFailures []time.Time               // recent wrong codes, oldest first
}

func NewAuthService(s *store.Store) *AuthService {
	return &AuthService{store: s, challenges: map[string]*totpChallenge{}}
}

func (a *AuthService) SetupRequired() (bool, error) {
	_, exists, err := a.store.GetSetting("admin_username")
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (a *AuthService) Setup(username, password string) error {
	required, err := a.SetupRequired()
	if err != nil {
		return err
	}
	if !required {
		return ErrSetupAlreadyDone
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := a.store.SetSetting("admin_username", username); err != nil {
		return err
	}
	return a.store.SetSetting("admin_password_hash", string(hash))
}

func (a *AuthService) Login(username, password string) (LoginResult, error) {
	storedUsername, ok, err := a.store.GetSetting("admin_username")
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || storedUsername != username {
		return LoginResult{}, ErrInvalidCredentials
	}

	hash, ok, err := a.store.GetSetting("admin_password_hash")
	if err != nil {
		return LoginResult{}, err
	}
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	enabled, err := a.TOTPEnabled()
	if err != nil {
		return LoginResult{}, err
	}
	if enabled {
		challenge, err := a.newTOTPChallenge()
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{TOTPChallenge: challenge}, nil
	}

	token, err := a.createSession()
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token}, nil
}

// CompleteTOTPLogin exchanges a challenge from Login plus a current code
// for a session token.
func (a *AuthService) CompleteTOTPLogin(challenge, code string) (token string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := hashToken(challenge)
	c, ok := a.challenges[key]
	if !ok || time.Now().After(c.expiresAt) {
		delete(a.challenges, key)
		return "", ErrTOTPChallengeInvalid
	}

	secret, ok, err := a.store.GetSetting(settingTOTPSecret)
	if err != nil {
		return "", err
	}
	if !ok {
		// Two-factor was turned off (e.g. by reset-password) mid-login.
		delete(a.challenges, key)
		return "", ErrTOTPChallengeInvalid
	}

	if err := a.verifyTOTPLocked(secret, code); err != nil {
		if errors.Is(err, ErrTOTPLocked) {
			return "", err
		}
		c.attempts++
		if c.attempts >= totpChallengeMaxAttempts {
			delete(a.challenges, key)
			return "", ErrTOTPChallengeInvalid
		}
		return "", err
	}

	delete(a.challenges, key)
	return a.createSession()
}

func (a *AuthService) TOTPEnabled() (bool, error) {
	_, ok, err := a.store.GetSetting(settingTOTPSecret)
	return ok, err
}

// BeginTOTPSetup generates a fresh secret and parks it as pending — it only
// takes effect once EnableTOTP sees a valid code from it, so a half-finished
// setup can never lock anyone out.
func (a *AuthService) BeginTOTPSetup(password string) (secret, uri string, err error) {
	if err := a.checkPassword(password); err != nil {
		return "", "", err
	}
	enabled, err := a.TOTPEnabled()
	if err != nil {
		return "", "", err
	}
	if enabled {
		return "", "", ErrTOTPAlreadyEnabled
	}
	username, _, err := a.store.GetSetting("admin_username")
	if err != nil {
		return "", "", err
	}

	secret, err = newTOTPSecret()
	if err != nil {
		return "", "", err
	}
	if err := a.store.SetSetting(settingTOTPPendingSecret, secret); err != nil {
		return "", "", err
	}
	return secret, totpURI(secret, username), nil
}

// EnableTOTP confirms the pending secret and signs out every session except
// keepToken (the one doing the enabling), so anything that got in on the
// password alone has to come back through the second factor.
func (a *AuthService) EnableTOTP(code, keepToken string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	enabled, err := a.TOTPEnabled()
	if err != nil {
		return err
	}
	if enabled {
		return ErrTOTPAlreadyEnabled
	}
	secret, ok, err := a.store.GetSetting(settingTOTPPendingSecret)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTOTPSetupNotStarted
	}

	step, ok := totpMatch(secret, code, time.Now(), 0)
	if !ok {
		return ErrInvalidTOTPCode
	}
	if err := a.store.SetSetting(settingTOTPLastStep, strconv.FormatUint(step, 10)); err != nil {
		return err
	}
	if err := a.store.SetSetting(settingTOTPSecret, secret); err != nil {
		return err
	}
	if err := a.store.DeleteSetting(settingTOTPPendingSecret); err != nil {
		return err
	}
	return a.store.DeleteSessionsExcept(hashToken(keepToken))
}

// DisableTOTP needs both the password and a current code, so a hijacked
// session alone can't strip the second factor off the account.
func (a *AuthService) DisableTOTP(password, code string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.checkPassword(password); err != nil {
		return err
	}

	secret, ok, err := a.store.GetSetting(settingTOTPSecret)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTOTPNotEnabled
	}
	if err := a.verifyTOTPLocked(secret, code); err != nil {
		return err
	}
	return a.clearTOTP()
}

// verifyTOTPLocked checks code and records its step as used, counting wrong
// codes toward the account-wide lockout. Caller holds a.mu.
func (a *AuthService) verifyTOTPLocked(secret, code string) error {
	now := time.Now()
	recent := a.totpFailures[:0]
	for _, t := range a.totpFailures {
		if now.Sub(t) < totpFailureWindow {
			recent = append(recent, t)
		}
	}
	a.totpFailures = recent
	if len(a.totpFailures) >= totpMaxFailures {
		return ErrTOTPLocked
	}

	var lastStep uint64
	if v, ok, err := a.store.GetSetting(settingTOTPLastStep); err != nil {
		return err
	} else if ok {
		lastStep, _ = strconv.ParseUint(v, 10, 64)
	}

	step, ok := totpMatch(secret, code, now, lastStep)
	if !ok {
		a.totpFailures = append(a.totpFailures, now)
		return ErrInvalidTOTPCode
	}
	return a.store.SetSetting(settingTOTPLastStep, strconv.FormatUint(step, 10))
}

func (a *AuthService) checkPassword(password string) error {
	hash, exists, err := a.store.GetSetting("admin_password_hash")
	if err != nil {
		return err
	}
	if !exists {
		return ErrNoAdminAccount
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrCurrentPasswordIncorrect
	}
	return nil
}

func (a *AuthService) clearTOTP() error {
	for _, key := range []string{settingTOTPSecret, settingTOTPLastStep, settingTOTPPendingSecret} {
		if err := a.store.DeleteSetting(key); err != nil {
			return err
		}
	}
	return nil
}

func (a *AuthService) newTOTPChallenge() (string, error) {
	challenge, key, err := newSessionToken()
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for k, c := range a.challenges {
		if now.After(c.expiresAt) {
			delete(a.challenges, k)
		}
	}
	a.challenges[key] = &totpChallenge{expiresAt: now.Add(totpChallengeTTL)}
	return challenge, nil
}

func (a *AuthService) createSession() (string, error) {
	_ = a.store.DeleteExpiredSessions()

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(sessionTTL).UTC().Format(time.RFC3339)
	if err := a.store.CreateSession(tokenHash, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword sets a new password for the existing admin account, turns
// off two-factor authentication and invalidates every active session — used
// by the `caddy-ui reset-password` CLI command, for when someone's locked
// out of the dashboard (lost password or lost authenticator alike; shell
// access to the server is the recovery factor).
func (a *AuthService) ResetPassword(password string) error {
	_, exists, err := a.store.GetSetting("admin_username")
	if err != nil {
		return err
	}
	if !exists {
		return ErrNoAdminAccount
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := a.store.SetSetting("admin_password_hash", string(hash)); err != nil {
		return err
	}
	if err := a.clearTOTP(); err != nil {
		return err
	}
	return a.store.DeleteAllSessions()
}

// ChangePassword lets a logged-in admin set a new password themselves,
// given their current one — unlike ResetPassword (the CLI recovery path),
// it does not invalidate other sessions, since this is a routine change,
// not a "something's wrong, kill everything" recovery.
func (a *AuthService) ChangePassword(currentPassword, newPassword string) error {
	hash, exists, err := a.store.GetSetting("admin_password_hash")
	if err != nil {
		return err
	}
	if !exists {
		return ErrNoAdminAccount
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)); err != nil {
		return ErrCurrentPasswordIncorrect
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return a.store.SetSetting("admin_password_hash", string(newHash))
}

func (a *AuthService) Logout(token string) error {
	return a.store.DeleteSession(hashToken(token))
}

func (a *AuthService) ValidateSession(token string) bool {
	if token == "" {
		return false
	}
	valid, err := a.store.SessionValid(hashToken(token))
	if err != nil {
		return false
	}
	return valid
}

func newSessionToken() (token, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
