package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"caddy-proxy-ui/internal/store"
)

var ErrSetupAlreadyDone = errors.New("admin account already exists")
var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrNoAdminAccount = errors.New("no administrator account exists yet — complete first-run setup via the web UI first")

const sessionTTL = 30 * 24 * time.Hour

// AuthService owns the single-admin account and its sessions. There is no
// default account and no default password — Setup must run once before
// anything else in the API is usable.
type AuthService struct {
	store *store.Store
}

func NewAuthService(s *store.Store) *AuthService {
	return &AuthService{store: s}
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

func (a *AuthService) Login(username, password string) (token string, err error) {
	storedUsername, ok, err := a.store.GetSetting("admin_username")
	if err != nil {
		return "", err
	}
	if !ok || storedUsername != username {
		return "", ErrInvalidCredentials
	}

	hash, ok, err := a.store.GetSetting("admin_password_hash")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

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

// ResetPassword sets a new password for the existing admin account and
// invalidates every active session — used by the `caddy-ui reset-password`
// CLI command, for when someone's locked out of the dashboard.
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
	return a.store.DeleteAllSessions()
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
