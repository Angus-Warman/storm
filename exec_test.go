package storm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdate_SingleColumn(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	created, _ := s.Create(RecordTextID{ID: "a", Value: "old"})

	n, err := s.Update[RecordTextID]().
		Where("ID = ?", created.ID).
		Set("Value", "new").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	got, err := s.Find(RecordTextID{ID: "a"})
	require.NoError(t, err)
	require.Equal(t, "new", got.Value)
}

func TestUpdate_MultipleColumns(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "x", Value: "v1"})

	n, err := s.Update[RecordTextID]().
		Where("ID = ?", "x").
		Set("ID", "y").
		Set("Value", "v2").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	got, err := s.Find(RecordTextID{ID: "y"})
	require.NoError(t, err)
	require.Equal(t, "v2", got.Value)
}

func TestUpdate_NoMatch(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "a", Value: "x"})

	n, err := s.Update[RecordTextID]().
		Where("ID = ?", "nonexistent").
		Set("Value", "new").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(0), n)
}

func TestUpdate_MultipleWheres(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "a", Value: "same"})
	s.Create(RecordTextID{ID: "b", Value: "same"})
	s.Create(RecordTextID{ID: "c", Value: "other"})

	n, err := s.Update[RecordTextID]().
		Where("Value = ?", "same").
		Where("ID != ?", "b").
		Set("Value", "updated").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	got, _ := s.Find(RecordTextID{ID: "a"})
	require.Equal(t, "updated", got.Value)

	got, _ = s.Find(RecordTextID{ID: "b"})
	require.Equal(t, "same", got.Value)
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "a", Value: "x"})
	s.Create(RecordTextID{ID: "b", Value: "y"})

	n, err := s.Delete[RecordTextID]().
		Where("ID = ?", "a").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	count, _ := s.Count[RecordTextID]()
	require.Equal(t, 1, count)
}

func TestDelete_NoMatch(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "a", Value: "x"})

	n, err := s.Delete[RecordTextID]().
		Where("ID = ?", "nonexistent").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(0), n)
}

func TestDelete_MultipleWheres(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "a", Value: "keep"})
	s.Create(RecordTextID{ID: "b", Value: "drop"})
	s.Create(RecordTextID{ID: "c", Value: "drop"})

	n, err := s.Delete[RecordTextID]().
		Where("Value = ? AND ID != ?", "drop", "c").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	count, _ := s.Count[RecordTextID]()
	require.Equal(t, 2, count)
}

func TestDelete_All(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "a", Value: "x"})
	s.Create(RecordTextID{ID: "b", Value: "y"})

	n, err := s.Delete[RecordTextID]().Run()
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	count, _ := s.Count[RecordTextID]()
	require.Equal(t, 0, count)
}

func TestThis(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())
	s.Create(RecordTextID{ID: "a", Value: "old"})

	n, err := s.Update[RecordTextID]().This(RecordTextID{ID: "a", Value: "new"})
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	got, err := s.Find(RecordTextID{ID: "a"})
	require.NoError(t, err)
	require.Equal(t, "new", got.Value)
}

func TestSQL_Update(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	sql := s.Update[RecordTextID]().
		Where("ID = ?").
		Set("Value", "new").
		SQL()
	require.Equal(t, "UPDATE RecordTextID SET Value = ? WHERE ID = ?;", sql)
}

func TestSQL_Delete(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	sql := s.Delete[RecordTextID]().
		Where("ID = ?").
		SQL()
	require.Equal(t, "DELETE FROM RecordTextID WHERE ID = ?;", sql)
}

func TestSQL_DeleteAll(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	sql := s.Delete[RecordTextID]().SQL()
	require.Equal(t, "DELETE FROM RecordTextID;", sql)
}

func TestSQL_MultipleSetsAndWheres(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordTextID]())

	sql := s.Update[RecordTextID]().
		Where("Value = ?").
		Where("ID = ?").
		Set("Value", "v1").
		Set("ID", "k1").
		SQL()
	require.Equal(t, "UPDATE RecordTextID SET Value = ?, ID = ? WHERE Value = ? AND ID = ?;", sql)
}

func TestUpdate_IntID(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordIntID]())
	created, _ := s.Create(RecordIntID{Value: "before"})

	n, err := s.Update[RecordIntID]().
		Where("ID = ?", created.ID).
		Set("Value", "after").
		Run()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	got, err := s.Find(RecordIntID{ID: created.ID})
	require.NoError(t, err)
	require.Equal(t, "after", got.Value)
}

func TestDeleteThis(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateTable[RecordIntID]())
	record, _ := s.Create(RecordIntID{Value: "first"})
	count, err := s.Get[RecordIntID]().Count()
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, err = s.Delete[RecordIntID]().This(record)
	count, err = s.Get[RecordIntID]().Count()
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
