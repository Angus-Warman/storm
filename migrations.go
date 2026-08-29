package gform

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

const migrationsTable = "__migrations"

// RunMigrations reads .sql files from fsys, executes any that have not yet been
// applied, and records them. All work is done inside a single transaction.
func (s *Store) RunMigrations(fsys fs.FS) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	if err := ensureMigrationsTable(tx); err != nil {
		return err
	}

	applied, err := appliedMigrations(tx)
	if err != nil {
		return err
	}

	files, err := collectSQLFiles(fsys)
	if err != nil {
		return err
	}

	pending := filterPending(files, applied)
	if len(pending) == 0 {
		return tx.Commit()
	}

	for _, name := range pending {
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			return fmt.Errorf("executing migration %s: %w", name, err)
		}

		if _, err := tx.Exec(
			fmt.Sprintf("INSERT INTO %s (name, applied_at) VALUES (?, ?);", migrationsTable),
			name, time.Now().UTC(),
		); err != nil {
			return fmt.Errorf("recording migration %s: %w", name, err)
		}
	}

	return tx.Commit()
}

func ensureMigrationsTable(tx *sql.Tx) error {
	_, err := tx.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		applied_at DATETIME NOT NULL
	);`, migrationsTable))
	return err
}

func appliedMigrations(tx *sql.Tx) (map[string]bool, error) {
	rows, err := tx.Query(fmt.Sprintf("SELECT name FROM %s;", migrationsTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func collectSQLFiles(fsys fs.FS) ([]string, error) {
	var files []string

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".sql") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		di, fi := path.Split(files[i])
		dj, fj := path.Split(files[j])
		if di != dj {
			return di < dj
		}
		return fi < fj
	})

	return files, nil
}

func filterPending(files []string, applied map[string]bool) []string {
	var out []string
	for _, f := range files {
		if !applied[f] {
			out = append(out, f)
		}
	}
	return out
}
