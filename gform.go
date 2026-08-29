package gform

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)

	if err != nil {
		return nil, err
	}

	return &Store{
		DB: db,
	}, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}
