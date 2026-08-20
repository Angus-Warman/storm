package storm

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

type RecordTime struct {
	ID      string
	Created time.Time
	Updated time.Time
}

func TestCreateTable_TextID(t *testing.T) {
	s := newTestStore(t)
	err := s.CreateTable[RecordTextID]()
	require.NoError(t, err)

	_, err = s.Create(RecordTextID{ID: "a", Value: "hello"})
	require.NoError(t, err)

	got, err := s.Find(RecordTextID{ID: "a"})
	require.NoError(t, err)
	require.Equal(t, "hello", got.Value)
}

func TestCreateTable_IntID(t *testing.T) {
	s := newTestStore(t)
	err := s.CreateTable[RecordIntID]()
	require.NoError(t, err)

	created, err := s.Create(RecordIntID{Value: "hello"})
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	got, err := s.Find(RecordIntID{ID: created.ID})
	require.NoError(t, err)
	require.Equal(t, "hello", got.Value)
}

func TestCreateTable_UUIDID(t *testing.T) {
	s := newTestStore(t)
	err := s.CreateTable[RecordUUID]()
	require.NoError(t, err)

	created, err := s.Create(RecordUUID{Value: "hello"})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil(), created.ID)

	got, err := s.Find(RecordUUID{ID: created.ID})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "hello", got.Value)
}

func TestCreateTable_NoIDField(t *testing.T) {
	type NoID struct {
		Name string
	}
	s := newTestStore(t)
	err := s.CreateTable[NoID]()
	require.Error(t, err)
	require.Contains(t, err.Error(), "ID")
}

func TestCreateTable_Idempotent(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	require.NoError(t, s.CreateTable[RecordTextID]())

	_, err := s.Create(RecordTextID{ID: "a", Value: "ok"})
	require.NoError(t, err)

	count, err := s.Count[RecordTextID]()
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCreateTable_TimeRoundTrip(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTime]())

	now := time.Now().UTC()
	_, err := s.Create(RecordTime{
		ID:      "t1",
		Created: now,
		Updated: now.Add(1 * time.Hour),
	})
	require.NoError(t, err)

	got, err := s.Find(RecordTime{ID: "t1"})
	require.NoError(t, err)
	require.Equal(t, now, got.Created)
	require.Equal(t, now.Add(1*time.Hour), got.Updated)
}

func TestCreateTable_MultipleColumnTypes(t *testing.T) {
	type MultiTypes struct {
		ID     string
		Name   string
		Age    int
		Score  float64
		Active bool
	}
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[MultiTypes]())

	_, err := s.Create(MultiTypes{
		ID:     "m1",
		Name:   "test",
		Age:    30,
		Score:  9.5,
		Active: true,
	})
	require.NoError(t, err)

	got, err := s.Find(MultiTypes{ID: "m1"})
	require.NoError(t, err)
	require.Equal(t, "test", got.Name)
	require.Equal(t, 30, got.Age)
	require.Equal(t, 9.5, got.Score)
	require.True(t, got.Active)
}

func TestCreateTables_MultipleTypes(t *testing.T) {
	s := newTestStore(t)
	err := s.CreateTables(RecordTextID{}, RecordIntID{}, RecordTime{})
	require.NoError(t, err)

	_, err = s.Create(RecordTextID{ID: "a", Value: "hello"})
	require.NoError(t, err)

	created, err := s.Create(RecordIntID{Value: "world"})
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	now := time.Now().UTC()
	_, err = s.Create(RecordTime{ID: "t1", Created: now, Updated: now})
	require.NoError(t, err)
}

func TestCreateTables_TransactionRollback(t *testing.T) {
	s := newTestStore(t)
	type HasNoIDea struct {
		Name string
	}
	err := s.CreateTables(RecordTextID{}, HasNoIDea{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ID")

	// The transaction must have been rolled back: RecordTextID's table should
	// not exist at all, not merely be empty.
	exists, err := tableExists(s.DB, "RecordTextID")
	require.NoError(t, err)
	require.False(t, exists, "RecordTextID table should not exist after rollback")
}

func TestCreateTables_DuplicateType(t *testing.T) {
	// Passing the same struct twice should be idempotent: the second call hits
	// migrateTable with an identical schema and must not error.
	s := newTestStore(t)
	err := s.CreateTables(RecordTextID{}, RecordTextID{})
	require.NoError(t, err)

	_, err = s.Create(RecordTextID{ID: "a", Value: "ok"})
	require.NoError(t, err)

	count, err := s.Count[RecordTextID]()
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCreateTables_PointerArg(t *testing.T) {
	// CreateTables must accept pointer-to-struct args as documented.
	s := newTestStore(t)
	err := s.CreateTables(&RecordTextID{})
	require.NoError(t, err)

	_, err = s.Create(RecordTextID{ID: "a", Value: "ptr"})
	require.NoError(t, err)
}

type RecordWithBlob struct {
	ID   string
	Data []byte
}

type RecordWithBlobV2 struct {
	ID    string
	Data  []byte
	Extra []byte
}

func TestEnsureTable_AdditiveMigration_BlobColumn(t *testing.T) {
	// Verifies that additive migration works correctly for BLOB columns,
	// exercising the [N]byte → BLOB path that previously emitted "UUID".
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordWithBlob]())

	payload := []byte("binary data")
	_, err := s.Create(RecordWithBlob{ID: "b1", Data: payload})
	require.NoError(t, err)

	// Simulate schema evolution by renaming the table.
	_, err = s.DB.Exec("ALTER TABLE RecordWithBlob RENAME TO RecordWithBlobV2;")
	require.NoError(t, err)

	// V2 adds an Extra BLOB column — must succeed.
	require.NoError(t, s.CreateTable[RecordWithBlobV2]())

	got, err := s.Find(RecordWithBlobV2{ID: "b1"})
	require.NoError(t, err)
	require.Equal(t, payload, got.Data)
	require.Nil(t, got.Extra) // DEFAULT NULL for new BLOB column
}

type RecordTextIDV2 struct {
	ID    string
	Value string
	Tag   string
}

func TestEnsureTable_AdditiveMigration(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	_, err := s.Create(RecordTextID{ID: "a", Value: "hello"})
	require.NoError(t, err)

	// Simulate a modified struct by renaming the table
	_, err = s.DB.Exec("ALTER TABLE RecordTextID RENAME TO RecordTextIDV2;")
	require.NoError(t, err)

	// V2 adds Tag column — should succeed via additive migration.
	require.NoError(t, s.CreateTable[RecordTextIDV2]())

	// Existing data is still there.
	got, err := s.Find(RecordTextIDV2{ID: "a"})
	require.NoError(t, err)
	require.Equal(t, "hello", got.Value)
	require.Equal(t, "", got.Tag) // zero value for new column

	// Can insert with the new column.
	_, err = s.Create(RecordTextIDV2{ID: "b", Value: "world", Tag: "new"})
	require.NoError(t, err)

	got2, err := s.Find(RecordTextIDV2{ID: "b"})
	require.NoError(t, err)
	require.Equal(t, "new", got2.Tag)
}

type RecordTextIDSubtractive struct {
	ID string
}

func TestEnsureTable_SubtractiveMigration(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	// Simulate a modified struct by renaming the table
	_, err := s.DB.Exec("ALTER TABLE RecordTextID RENAME TO RecordTextIDSubtractive;")
	require.NoError(t, err)

	// Table has ID + Value but struct only has ID — subtractive, should error.
	err = s.CreateTable[RecordTextIDSubtractive]()
	require.Error(t, err)
	require.Contains(t, err.Error(), "subtractive")
}

func TestEnsureTable_IdempotentMigration(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextIDV2]())
	// Running again with the same schema should be a no-op.
	require.NoError(t, s.CreateTable[RecordTextIDV2]())
}
