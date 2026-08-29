package gform

import (
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// RecordTextIDRenamed is RecordTextID with "Value" renamed to "Label".
type RecordTextIDRenamed struct {
	ID    string
	Label string
}

func TestMigrations_RenameColumn(t *testing.T) {
	s := newTestStore(t)

	// 1. Create the original table and insert data.
	require.NoError(t, s.CreateTable[RecordTextID]())
	_, err := s.Create(RecordTextID{ID: "a", Value: "hello"})
	require.NoError(t, err)

	// 2. Simulate the user renaming the struct field by renaming the table.
	_, err = s.DB.Exec("ALTER TABLE RecordTextID RENAME TO RecordTextIDRenamed;")
	require.NoError(t, err)

	// 3. CreateTable should fail: table has "Value" but struct expects "Label".
	err = s.CreateTable[RecordTextIDRenamed]()
	require.Error(t, err)
	require.Contains(t, err.Error(), "subtractive")

	// 4. Run a migration that renames the column.
	fsys := os.DirFS("testdata/migrations_rename")
	require.NoError(t, s.RunMigrations(fsys))

	// 5. CreateTable should now succeed.
	require.NoError(t, s.CreateTable[RecordTextIDRenamed]())

	// 6. Data should be intact.
	got, err := s.Find(RecordTextIDRenamed{ID: "a"})
	require.NoError(t, err)
	require.Equal(t, "hello", got.Label)
}

func TestMigrations_Basic(t *testing.T) {
	s := newTestStore(t)

	fsys := os.DirFS("testdata/migrations")
	require.NoError(t, s.RunMigrations(fsys))

	// Tables should exist.
	var tableName string
	require.NoError(t, s.DB.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='users';",
	).Scan(&tableName))
	require.Equal(t, "users", tableName)

	// __migrations should track all three files.
	count := 0
	rows, err := s.DB.Query("SELECT name FROM __migrations ORDER BY name;")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		count++
	}
	require.Equal(t, 3, count)
}

func TestMigrations_Idempotent(t *testing.T) {
	s := newTestStore(t)
	fsys := os.DirFS("testdata/migrations")

	require.NoError(t, s.RunMigrations(fsys))
	require.NoError(t, s.RunMigrations(fsys))

	count := 0
	rows, err := s.DB.Query("SELECT name FROM __migrations;")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		count++
	}
	require.Equal(t, 3, count)
}

func TestMigrations_TransactionRollback(t *testing.T) {
	s := newTestStore(t)

	badFS := os.DirFS("testdata/bad_migrations")
	err := s.RunMigrations(badFS)
	require.Error(t, err)

	// __migrations table should not exist because the transaction rolled back.
	var exists bool
	err = s.DB.QueryRow(
		"SELECT COUNT(*) > 0 FROM sqlite_master WHERE type='table' AND name='__migrations';",
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestMigrations_Empty(t *testing.T) {
	s := newTestStore(t)

	emptyFS := os.DirFS("testdata/empty_migrations")
	require.NoError(t, s.RunMigrations(emptyFS))

	// __migrations table should still be created.
	var exists bool
	err := s.DB.QueryRow(
		"SELECT COUNT(*) > 0 FROM sqlite_master WHERE type='table' AND name='__migrations';",
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestMigrations_DeterministicOrder(t *testing.T) {
	s := newTestStore(t)
	fsys := os.DirFS("testdata/migrations")

	require.NoError(t, s.RunMigrations(fsys))

	// Read applied names in order.
	var names []string
	rows, err := s.DB.Query("SELECT name FROM __migrations ORDER BY id;")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}

	require.Equal(t, []string{
		"001_create_users.sql",
		"002_create_posts.sql",
		"sub/003_add_index.sql",
	}, names)
}

func TestMigrations_Incremental(t *testing.T) {
	s := newTestStore(t)

	// Only apply first migration.
	singleFS := os.DirFS("testdata/migrations_single")
	require.NoError(t, s.RunMigrations(singleFS))

	var tableName string
	err := s.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users';").Scan(&tableName)
	require.NoError(t, err)
	require.Equal(t, "users", tableName)

	// posts table should not exist.
	err = s.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='posts';").Scan(&tableName)
	require.ErrorIs(t, err, sql.ErrNoRows)

	// Now apply the full set.
	fullFS := os.DirFS("testdata/migrations")
	require.NoError(t, s.RunMigrations(fullFS))

	// posts table should now exist.
	err = s.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='posts';").Scan(&tableName)
	require.NoError(t, err)
	require.Equal(t, "posts", tableName)
}
