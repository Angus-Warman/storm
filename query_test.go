package gform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func seedItems(t *testing.T, s *Store) {
	t.Helper()
	require.NoError(t, s.CreateTable[RecordTextID]())
	for _, item := range []RecordTextID{
		{ID: "alpha", Value: "first"},
		{ID: "bravo", Value: "second"},
		{ID: "charlie", Value: "third"},
		{ID: "delta", Value: "fourth"},
	} {
		_, err := s.Create(item)
		require.NoError(t, err)
	}
}

func TestQueryAll_NoFilters(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	results, err := s.Get[RecordTextID]().All()
	require.NoError(t, err)
	require.Len(t, results, 4)
}

func TestQueryOne_NoFilters(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordIntID]())
	_, err := s.Create(RecordIntID{Value: "only"})
	require.NoError(t, err)

	item, err := s.Get[RecordIntID]().One()
	require.NoError(t, err)
	require.Equal(t, "only", item.Value)
}

func TestQueryOne_NoRows(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	_, err := s.Get[RecordTextID]().One()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no RecordTextID found")
}

func TestQueryWhere_Single(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	results, err := s.Get[RecordTextID]().Where("Value = ?", "second").All()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "bravo", results[0].ID)
}

func TestQueryWhere_Multiple(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	results, err := s.Get[RecordTextID]().
		Where("Value = ?", "first").
		Where("ID = ?", "alpha").
		All()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "alpha", results[0].ID)
}

func TestQueryWhere_NoMatch(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	results, err := s.Get[RecordTextID]().Where("Value = ?", "nonexistent").All()
	require.NoError(t, err)
	require.Len(t, results, 0)
}

func TestQueryOrderBy(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	results, err := s.Get[RecordTextID]().OrderBy("ID DESC").All()
	require.NoError(t, err)
	require.Len(t, results, 4)
	require.Equal(t, "delta", results[0].ID)
	require.Equal(t, "charlie", results[1].ID)
	require.Equal(t, "bravo", results[2].ID)
	require.Equal(t, "alpha", results[3].ID)
}

func TestQueryFirst(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	item, err := s.Get[RecordTextID]().First()
	require.NoError(t, err)
	require.Equal(t, "alpha", item.ID)
}

func TestQueryLast(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	item, err := s.Get[RecordTextID]().Last()
	require.NoError(t, err)
	require.Equal(t, "delta", item.ID)
}

func TestQueryFirst_NoRows(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	_, err := s.Get[RecordTextID]().First()
	require.Error(t, err)
}

func TestQueryLast_NoRows(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	_, err := s.Get[RecordTextID]().Last()
	require.Error(t, err)
}

func TestQueryFirst_WithWhere(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	item, err := s.Get[RecordTextID]().
		Where("ID IN (?, ?)", "charlie", "alpha").
		First()
	require.NoError(t, err)
	require.Equal(t, "alpha", item.ID)
}

func TestQueryLast_WithWhere(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	item, err := s.Get[RecordTextID]().
		Where("ID IN (?, ?)", "alpha", "charlie").
		Last()
	require.NoError(t, err)
	require.Equal(t, "charlie", item.ID)
}

func TestQueryOne_MultipleRows(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	_, err := s.Get[RecordTextID]().Where("ID IN (?, ?)", "alpha", "bravo").One()
	// One with multiple results still returns a row (SQLite doesn't enforce LIMIT 1)
	require.NoError(t, err)
}

func TestQuerySQL(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	sql := s.Get[RecordTextID]().
		Where("ID = ?", "test").
		OrderBy("Value ASC").
		ToSQL()
	require.Equal(t, "SELECT * FROM RecordTextID WHERE ID = ? ORDER BY Value ASC;", sql)
}

func TestQuerySQL_NoClauses(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	sql := s.Get[RecordTextID]().ToSQL()
	require.Equal(t, "SELECT * FROM RecordTextID;", sql)
}

func TestQueryWhere_WithLike(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	results, err := s.Get[RecordTextID]().
		Where("ID LIKE ?", "c%").
		All()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "charlie", results[0].ID)
}

func TestQueryWhere_WithBetween(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	results, err := s.Get[RecordTextID]().
		Where("rowid BETWEEN ? AND ?", 2, 3).
		All()
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestQueryChaining(t *testing.T) {
	s := newTestStore(t)
	seedItems(t, s)

	item, err := s.Get[RecordTextID]().
		Where("ID != ?", "alpha").
		Where("ID != ?", "bravo").
		OrderBy("ID ASC").
		First()
	require.NoError(t, err)
	require.Equal(t, "charlie", item.ID)
}

func TestQueryIntID_OrderBy(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordIntID]())

	for _, v := range []string{"first", "second", "third"} {
		_, err := s.Create(RecordIntID{Value: v})
		require.NoError(t, err)
	}

	item, err := s.Get[RecordIntID]().First()
	require.NoError(t, err)
	require.Equal(t, "first", item.Value)

	item, err = s.Get[RecordIntID]().Last()
	require.NoError(t, err)
	require.Equal(t, "third", item.Value)
}

func TestQueryIntID_Where(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordIntID]())

	created, _ := s.Create(RecordIntID{Value: "target"})
	_, _ = s.Create(RecordIntID{Value: "other"})

	item, err := s.Get[RecordIntID]().
		Where("ID = ?", created.ID).
		One()
	require.NoError(t, err)
	require.Equal(t, "target", item.Value)
}
