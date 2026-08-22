package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicateDomain = errors.New("domain already exists")

type Host struct {
	ID             int64
	Domain         string
	Upstream       string
	RequestHeaders string // raw JSON object, e.g. {"X-Foo":"bar"}
	Enabled        bool
	CreatedAt      string
	UpdatedAt      string
}

func (s *Store) ListHosts() ([]Host, error) {
	rows, err := s.db.Query(`SELECT id, domain, upstream, request_headers, enabled, created_at, updated_at
		FROM hosts ORDER BY domain`)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	defer rows.Close()

	var hosts []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.Domain, &h.Upstream, &h.RequestHeaders, &h.Enabled, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan host: %w", err)
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *Store) GetHost(id int64) (Host, error) {
	var h Host
	err := s.db.QueryRow(`SELECT id, domain, upstream, request_headers, enabled, created_at, updated_at
		FROM hosts WHERE id = ?`, id).
		Scan(&h.ID, &h.Domain, &h.Upstream, &h.RequestHeaders, &h.Enabled, &h.CreatedAt, &h.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, ErrNotFound
	}
	if err != nil {
		return Host{}, fmt.Errorf("get host: %w", err)
	}
	return h, nil
}

func (s *Store) CreateHost(domain, upstream, requestHeaders string, enabled bool) (Host, error) {
	res, err := s.db.Exec(`INSERT INTO hosts (domain, upstream, request_headers, enabled) VALUES (?, ?, ?, ?)`,
		domain, upstream, requestHeaders, enabled)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return Host{}, ErrDuplicateDomain
		}
		return Host{}, fmt.Errorf("create host: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Host{}, fmt.Errorf("create host: %w", err)
	}
	return s.GetHost(id)
}

func (s *Store) UpdateHost(id int64, domain, upstream, requestHeaders string, enabled bool) (Host, error) {
	res, err := s.db.Exec(`UPDATE hosts SET domain = ?, upstream = ?, request_headers = ?, enabled = ?,
		updated_at = datetime('now') WHERE id = ?`, domain, upstream, requestHeaders, enabled, id)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return Host{}, ErrDuplicateDomain
		}
		return Host{}, fmt.Errorf("update host: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Host{}, fmt.Errorf("update host: %w", err)
	}
	if n == 0 {
		return Host{}, ErrNotFound
	}
	return s.GetHost(id)
}

func (s *Store) SetHostEnabled(id int64, enabled bool) (Host, error) {
	res, err := s.db.Exec(`UPDATE hosts SET enabled = ?, updated_at = datetime('now') WHERE id = ?`, enabled, id)
	if err != nil {
		return Host{}, fmt.Errorf("toggle host: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Host{}, fmt.Errorf("toggle host: %w", err)
	}
	if n == 0 {
		return Host{}, ErrNotFound
	}
	return s.GetHost(id)
}

func (s *Store) DeleteHost(id int64) error {
	res, err := s.db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete host: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete host: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
