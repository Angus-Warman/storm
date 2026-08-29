package gform

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

type dblike interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
}

// CreateTable creates a SQLite table for the given type T.
// The type must have a field named "ID" which will be used as the primary key.
func (s *Store) CreateTable[T any]() error {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return ensureTableForStruct(s.DB, t)
}

// CreateTables creates SQLite tables for all provided struct types inside a single transaction.
// Each argument must be a struct value (or pointer to struct) that has an ID field.
func (s *Store) CreateTables(structs ...any) error {
	if len(structs) == 0 {
		return fmt.Errorf("no tables to create")
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, v := range structs {
		t := reflect.TypeOf(v)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return fmt.Errorf("type %s is not a struct", t.Name())
		}
		if err := ensureTableForStruct(tx, t); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ensureTableForStruct checks whether the table exists. If absent it creates it.
// If present it validates the schema and applies any additive migrations.
func ensureTableForStruct(db dblike, t reflect.Type) error {
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("type %s is not a struct", t.Name())
	}

	if _, hasID := t.FieldByName("ID"); !hasID {
		return fmt.Errorf("type %s has no field named 'ID': an ID field is required as the primary key", t.Name())
	}

	tableName := t.Name()
	exists, err := tableExists(db, tableName)
	if err != nil {
		return fmt.Errorf("checking table %s: %w", tableName, err)
	}

	if !exists {
		return createTable(db, t)
	}

	return migrateTable(db, t)
}

func tableExists(db dblike, name string) (bool, error) {
	rows, err := db.Query("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?;", name)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, rows.Err()
}

func createTable(db dblike, t reflect.Type) error {
	idField, _ := t.FieldByName("ID")
	tableName := t.Name()
	var columns []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		sqlType, err := goTypeToSQLite(field.Type)
		if err != nil {
			return fmt.Errorf("unsupported type for field %s: %w", field.Name, err)
		}

		colDef := fmt.Sprintf("%s %s", field.Name, sqlType)

		if field.Name == "ID" {
			var pkType string
			switch idField.Type.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				pkType = "INTEGER PRIMARY KEY"
			case reflect.String:
				pkType = "TEXT PRIMARY KEY"
			case reflect.Array:
				if idField.Type.Elem().Kind() == reflect.Uint8 {
					pkType = "BLOB PRIMARY KEY"
				} else {
					return fmt.Errorf("unsupported ID field type: %s (must be integer, string, or [N]byte)", idField.Type.Kind())
				}
			default:
				return fmt.Errorf("unsupported ID field type: %s (must be integer or string)", idField.Type.Kind())
			}
			colDef = fmt.Sprintf("ID %s", pkType)
		}

		columns = append(columns, colDef)
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n);",
		tableName,
		strings.Join(columns, ",\n  "),
	)

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	return nil
}

// columnInfo holds a row from PRAGMA table_info.
type columnInfo struct {
	name    string
	sqlType string
}

func tableColumns(db dblike, tableName string) ([]columnInfo, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []columnInfo
	for rows.Next() {
		var ci columnInfo
		var cid int
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &ci.name, &ci.sqlType, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, ci)
	}
	return cols, rows.Err()
}

func migrateTable(db dblike, t reflect.Type) error {
	tableName := t.Name()

	// Use the passed-in db (may be a transaction) so schema reads are consistent
	// with any DDL already executed within the same transaction.
	currentCols, err := tableColumns(db, tableName)
	if err != nil {
		return fmt.Errorf("reading schema for %s: %w", tableName, err)
	}

	currentByName := make(map[string]string, len(currentCols))
	for _, c := range currentCols {
		currentByName[c.name] = c.sqlType
	}

	// Check for subtractive changes: every current column must still exist in the struct.
	for colName := range currentByName {
		if _, ok := t.FieldByName(colName); !ok {
			return fmt.Errorf("table %s has column %s which is not present in struct %s: subtractive migrations are not supported", tableName, colName, t.Name())
		}
	}

	// Apply additive migrations: any struct field not yet in the table.
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if _, exists := currentByName[field.Name]; exists {
			continue
		}

		sqlType, err := goTypeToSQLite(field.Type)
		if err != nil {
			return fmt.Errorf("unsupported type for field %s: %w", field.Name, err)
		}

		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s DEFAULT %s;",
			tableName, field.Name, sqlType, sqlDefault(sqlType))
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("adding column %s to %s: %w", field.Name, tableName, err)
		}
	}

	return nil
}

func sqlDefault(sqlType string) string {
	switch sqlType {
	case "INTEGER":
		return "0"
	case "REAL":
		return "0"
	case "TEXT":
		return "''"
	case "DATETIME":
		return "'0001-01-01T00:00:00Z'"
	case "BLOB":
		return "NULL"
	default:
		return "NULL"
	}
}

func goTypeToSQLite(t reflect.Type) (string, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Bool:
		return "INTEGER", nil
	case reflect.Float32, reflect.Float64:
		return "REAL", nil
	case reflect.String:
		return "TEXT", nil
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "BLOB", nil // []byte
		}
		return "", fmt.Errorf("unsupported slice type: %s", t)
	case reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return "BLOB", nil // [N]byte, e.g. UUID
		}
		return "", fmt.Errorf("unsupported array type: %s", t)
	default:
		// Check for time.Time
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return "DATETIME", nil
		}
		return "", fmt.Errorf("unsupported type: %s", t.Kind())
	}
}
