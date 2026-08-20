package storm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"uuid"
)

// panics if T is not a struct
func getStructType[T any]() reflect.Type {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Errorf("type %s is not a struct", t.Name()))
	}
	return t
}

func extractID(v reflect.Value) (any, error) {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	idField := v.FieldByName("ID")
	if !idField.IsValid() {
		return nil, fmt.Errorf("type %s has no field named 'ID'", v.Type().Name())
	}
	return idField.Interface(), nil
}

// nonIDFields returns all fields except ID, in order.
func nonIDFields(t reflect.Type) []reflect.StructField {
	var fields []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name != "ID" {
			fields = append(fields, f)
		}
	}
	return fields
}

// panics if T is not a struct
func (s *Store) Count[T any]() (int, error) {
	t := getStructType[T]()

	row := s.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s;", t.Name()))
	count := 0
	err := row.Scan(&count)

	return count, err
}

// Find uses the ID property of item.
// panics if T is not a struct.
func (s *Store) Find[T any](item T) (T, error) {
	t := getStructType[T]()

	id, err := extractID(reflect.ValueOf(item))
	if err != nil {
		var zero T
		return zero, err
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE ID = ?;", t.Name())
	row := s.DB.QueryRow(query, id)

	var blank T
	v := reflect.ValueOf(&blank).Elem()
	ptrs := make([]any, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		ptrs[i] = v.Field(i).Addr().Interface()
	}

	if err := row.Scan(ptrs...); err != nil {
		if err == sql.ErrNoRows {
			return blank, fmt.Errorf("no %s found with ID %v", t.Name(), id)
		}
		return blank, err
	}
	return blank, nil
}

// panics if T is not a struct
func (s *Store) Create[T any](item T) (T, error) {
	t := getStructType[T]()

	v := reflect.New(t).Elem()
	v.Set(reflect.ValueOf(item))

	idField, _ := t.FieldByName("ID")
	isAutoIncrement := isIntegerKind(idField.Type)
	isUUID := idField.Type == reflect.TypeOf(uuid.UUID{})

	// Auto-generate UUID if zero
	if isUUID {
		idVal := v.FieldByName("ID")
		if idVal.IsZero() {
			idVal.Set(reflect.ValueOf(uuid.NewV7()))
		}
	}

	var allFields []reflect.StructField
	if isAutoIncrement {
		allFields = nonIDFields(t)
	} else {
		allFields = make([]reflect.StructField, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			allFields = append(allFields, t.Field(i))
		}
	}

	colNames := make([]string, len(allFields))
	placeholders := make([]string, len(allFields))
	values := make([]any, len(allFields))

	for i, f := range allFields {
		colNames[i] = f.Name
		placeholders[i] = "?"
		values[i] = v.FieldByName(f.Name).Interface()
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		t.Name(),
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "),
	)

	res, err := s.DB.Exec(query, values...)
	if err != nil {
		return item, err
	}

	out := reflect.New(t).Elem()
	out.Set(v)

	if isAutoIncrement {
		newID, err := res.LastInsertId()
		if err != nil {
			return item, err
		}
		idField := out.FieldByName("ID")
		if idField.CanSet() {
			idField.Set(reflect.ValueOf(newID).Convert(idField.Type()))
		}
	}

	return out.Interface().(T), nil
}

func isIntegerKind(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}
