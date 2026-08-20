package storm

import (
	"fmt"
	"reflect"
	"strings"
)

type ExecBuilder[T any] struct {
	store  *Store
	type_  reflect.Type
	sets   []setClause
	wheres []whereClause
	del    bool
}

type setClause struct {
	column string
	value  any
}

// panics if T is not a struct
func (s *Store) Update[T any]() *ExecBuilder[T] {
	t := getStructType[T]()
	return &ExecBuilder[T]{
		store: s,
		type_: t,
	}
}

// panics if T is not a struct
func (s *Store) Delete[T any]() *ExecBuilder[T] {
	t := getStructType[T]()
	return &ExecBuilder[T]{
		store: s,
		type_: t,
		del:   true,
	}
}

func (e *ExecBuilder[T]) Where(condition string, args ...any) *ExecBuilder[T] {
	e.wheres = append(e.wheres, whereClause{condition: condition, args: args})
	return e
}

func (e *ExecBuilder[T]) Set(column string, value any) *ExecBuilder[T] {
	e.sets = append(e.sets, setClause{column: column, value: value})
	return e
}

// This is a terminator. It sets the ID for the WHERE clause and all non-ID
// fields for SET, then executes the statement.
func (e *ExecBuilder[T]) This(item T) (int64, error) {
	id, err := extractID(reflect.ValueOf(item))
	if err != nil {
		return 0, err
	}
	if !e.del {
		fields := nonIDFields(e.type_)
		v := reflect.ValueOf(item)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		for _, f := range fields {
			e.sets = append(e.sets, setClause{column: f.Name, value: v.FieldByName(f.Name).Interface()})
		}
	}
	e.wheres = append(e.wheres, whereClause{condition: "ID = ?", args: []any{id}})
	return e.Run()
}

func (e *ExecBuilder[T]) Run() (int64, error) {
	query := e.SQL()
	var allArgs []any
	for _, s := range e.sets {
		allArgs = append(allArgs, s.value)
	}
	for _, w := range e.wheres {
		allArgs = append(allArgs, w.args...)
	}
	res, err := e.store.DB.Exec(query, allArgs...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (e *ExecBuilder[T]) SQL() string {
	var sb strings.Builder

	if len(e.sets) > 0 {
		sb.WriteString(fmt.Sprintf("UPDATE %s SET ", e.type_.Name()))
		clauses := make([]string, len(e.sets))
		for i, s := range e.sets {
			clauses[i] = fmt.Sprintf("%s = ?", s.column)
		}
		sb.WriteString(strings.Join(clauses, ", "))
	} else {
		sb.WriteString(fmt.Sprintf("DELETE FROM %s", e.type_.Name()))
	}

	if len(e.wheres) > 0 {
		sb.WriteString(" WHERE ")
		conditions := make([]string, len(e.wheres))
		for i, w := range e.wheres {
			conditions[i] = w.condition
		}
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	sb.WriteString(";")
	return sb.String()
}
