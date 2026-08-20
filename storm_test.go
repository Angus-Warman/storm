package storm

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
)

type RecordTextID struct {
	ID    string
	Value string
}

type RecordIntID struct {
	ID    int
	Value string
}

type RecordUUID struct {
	ID    uuid.UUID
	Value string
}

func TestTextID(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	err = s.CreateTable[RecordTextID]()
	require.NoError(t, err)

	_, err = s.Create(RecordTextID{ID: "first", Value: "post"})
	require.NoError(t, err)

	count, err := s.Count[RecordTextID]()
	require.NoError(t, err)
	require.Equal(t, 1, count)

	records, err := s.Get[RecordTextID]().All()
	require.NoError(t, err)

	require.Len(t, records, 1)
}

func TestIntID(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	err = s.CreateTable[RecordIntID]()
	require.NoError(t, err)

	created, err := s.Create(RecordIntID{Value: "post"})
	require.NoError(t, err)
	id := created.ID

	count, err := s.Count[RecordIntID]()
	require.NoError(t, err)
	require.Equal(t, 1, count)

	records, err := s.Get[RecordIntID]().All()
	require.NoError(t, err)

	require.Len(t, records, 1)

	_, err = s.Update[RecordIntID]().
		Where("ID = ?", id).
		Set("Value", "post2").
		Run()
	require.NoError(t, err)
}

type RecordForeign struct {
	ID       string
	Value    string
	ParentID uuid.UUID
}

func TestForeignUUID(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	err = s.CreateTable[RecordForeign]()
	require.NoError(t, err)

	parentID := uuid.NewV7()
	_, err = s.Create(RecordForeign{ID: "child", Value: "data", ParentID: parentID})
	require.NoError(t, err)

	records, err := s.Get[RecordForeign]().All()
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, parentID, records[0].ParentID)
}

func TestUUID(t *testing.T) {
	s, err := New(":memory:")
	require.NoError(t, err)

	err = s.CreateTable[RecordUUID]()
	require.NoError(t, err)

	created, err := s.Create(RecordUUID{Value: "post"})
	require.NoError(t, err)
	id := created.ID
	require.NotEqual(t, uuid.Nil(), id) // If not already set, ID should initialise as newV7

	count, err := s.Count[RecordUUID]()
	require.NoError(t, err)
	require.Equal(t, 1, count)

	records, err := s.Get[RecordUUID]().All()
	require.NoError(t, err)

	require.Len(t, records, 1)

	_, err = s.Update[RecordUUID]().
		Where("ID = ?", id).
		Set("Value", "post2").
		Run()
	require.NoError(t, err)
}
