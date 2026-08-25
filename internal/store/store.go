// Package store provides SQLite-backed persistence. SQLite is the source of
// truth for the whole application; Caddy's running config is rebuilt from it.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite only supports one writer at a time; a single connection avoids
	// SQLITE_BUSY errors under this app's low, mostly-serial write volume.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// migrate applies schema changes CREATE TABLE IF NOT EXISTS can't express —
// i.e. new columns on a table that may already exist from a previous
// version. Each step must be safe to run against both a brand-new database
// (where schema.sql already has the column) and an existing one.
func migrate(db *sql.DB) error {
	hasColumn, err := columnExists(db, "hosts", "group_label")
	if err != nil {
		return err
	}
	if !hasColumn {
		if _, err := db.Exec(`ALTER TABLE hosts ADD COLUMN group_label TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add hosts.group_label: %w", err)
		}
	}
	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) Close() error {
	return s.db.Close()
}
