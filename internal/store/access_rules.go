package store

import (
	"fmt"
	"strings"
)

type AccessRuleKind string

const (
	AccessRuleBasicAuth AccessRuleKind = "basic_auth"
	AccessRuleIPAllow   AccessRuleKind = "ip_allow"
	AccessRuleIPDeny    AccessRuleKind = "ip_deny"
)

type AccessRule struct {
	ID        int64
	HostID    int64
	Kind      AccessRuleKind
	Value     string // basic_auth: JSON {"username","bcrypt_hash"}; ip_*: one CIDR/IP
	CreatedAt string
}

func (s *Store) ListAccessRules(hostID int64) ([]AccessRule, error) {
	rows, err := s.db.Query(`SELECT id, host_id, kind, value, created_at
		FROM host_access_rules WHERE host_id = ? ORDER BY id`, hostID)
	if err != nil {
		return nil, fmt.Errorf("list access rules: %w", err)
	}
	defer rows.Close()

	var rules []AccessRule
	for rows.Next() {
		var r AccessRule
		if err := rows.Scan(&r.ID, &r.HostID, &r.Kind, &r.Value, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan access rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// ListAccessRulesForHosts returns rules for every given host in one query,
// grouped by host ID — avoids an N+1 query when building the full Caddy
// config from all hosts at once.
func (s *Store) ListAccessRulesForHosts(hostIDs []int64) (map[int64][]AccessRule, error) {
	result := make(map[int64][]AccessRule, len(hostIDs))
	if len(hostIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(hostIDs))
	args := make([]any, len(hostIDs))
	for i, id := range hostIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT id, host_id, kind, value, created_at FROM host_access_rules
		WHERE host_id IN (%s) ORDER BY host_id, id`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list access rules for hosts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r AccessRule
		if err := rows.Scan(&r.ID, &r.HostID, &r.Kind, &r.Value, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan access rule: %w", err)
		}
		result[r.HostID] = append(result[r.HostID], r)
	}
	return result, rows.Err()
}

func (s *Store) CreateAccessRule(hostID int64, kind AccessRuleKind, value string) (AccessRule, error) {
	res, err := s.db.Exec(`INSERT INTO host_access_rules (host_id, kind, value) VALUES (?, ?, ?)`,
		hostID, kind, value)
	if err != nil {
		return AccessRule{}, fmt.Errorf("create access rule: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return AccessRule{}, fmt.Errorf("create access rule: %w", err)
	}
	var r AccessRule
	err = s.db.QueryRow(`SELECT id, host_id, kind, value, created_at FROM host_access_rules WHERE id = ?`, id).
		Scan(&r.ID, &r.HostID, &r.Kind, &r.Value, &r.CreatedAt)
	if err != nil {
		return AccessRule{}, fmt.Errorf("create access rule: %w", err)
	}
	return r, nil
}

func (s *Store) DeleteAccessRule(id int64) error {
	res, err := s.db.Exec(`DELETE FROM host_access_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete access rule: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete access rule: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
