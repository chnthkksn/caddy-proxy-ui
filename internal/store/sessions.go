package store

import (
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) CreateSession(tokenHash, expiresAt string) error {
	_, err := s.db.Exec(`INSERT INTO sessions (token_hash, expires_at) VALUES (?, ?)`, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionValid reports whether a non-expired session exists for tokenHash.
func (s *Store) SessionValid(tokenHash string) (bool, error) {
	var expiresAt string
	err := s.db.QueryRow(`SELECT expires_at FROM sessions WHERE token_hash = ?`, tokenHash).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check session: %w", err)
	}
	var expired bool
	err = s.db.QueryRow(`SELECT datetime(?) <= datetime('now')`, expiresAt).Scan(&expired)
	if err != nil {
		return false, fmt.Errorf("check session expiry: %w", err)
	}
	if expired {
		_ = s.DeleteSession(tokenHash)
		return false, nil
	}
	return true, nil
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions prunes stale rows; call opportunistically (e.g. on login).
func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE datetime(expires_at) <= datetime('now')`)
	if err != nil {
		return fmt.Errorf("prune sessions: %w", err)
	}
	return nil
}
