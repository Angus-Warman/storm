package storm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

type QueryBuilder[T any] struct {
	store   *Store
	type_   reflect.Type
	wheres  []whereClause
	orderBy string
}

type whereClause struct {
	condition string
	args      []any
}

// panics if T is not a struct
func (s *Store) Get[T any]() *QueryBuilder[T] {
	t := getStructType[T]()
	return &QueryBuilder[T]{
		store: s,
		type_: t,
	}
}

func (q *QueryBuilder[T]) Where(condition string, args ...any) *QueryBuilder[T] {
	q.wheres = append(q.wheres, whereClause{condition: condition, args: args})
	return q
}

func (q *QueryBuilder[T]) OrderBy(orderBy string) *QueryBuilder[T] {
	q.orderBy = orderBy
	return q
}

func (q *QueryBuilder[T]) addWhere(args ...any) {
	if len(args) > 0 {
		if condition, ok := args[0].(string); ok && condition != "" {
			q.wheres = append(q.wheres, whereClause{condition: condition, args: args[1:]})
		}
	}
}

func (q *QueryBuilder[T]) buildQuery() (string, []any) {
	var sb strings.Builder
	var allArgs []any

	sb.WriteString(fmt.Sprintf("SELECT * FROM %s", q.type_.Name()))

	if len(q.wheres) > 0 {
		sb.WriteString(" WHERE ")
		conditions := make([]string, len(q.wheres))
		for i, w := range q.wheres {
			conditions[i] = w.condition
			allArgs = append(allArgs, w.args...)
		}
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	if q.orderBy != "" {
		sb.WriteString(fmt.Sprintf(" ORDER BY %s", q.orderBy))
	}

	sb.WriteString(";")
	return sb.String(), allArgs
}

func (q *QueryBuilder[T]) scanRow(scanner interface{ Scan(dest ...any) error }) (T, error) {
	var item T
	v := reflect.ValueOf(&item).Elem()
	ptrs := make([]any, q.type_.NumField())
	for i := 0; i < q.type_.NumField(); i++ {
		ptrs[i] = v.Field(i).Addr().Interface()
	}
	if err := scanner.Scan(ptrs...); err != nil {
		return item, err
	}
	return item, nil
}

func (q *QueryBuilder[T]) All(args ...any) ([]T, error) {
	q.addWhere(args...)
	query, args := q.buildQuery()
	rows, err := q.store.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		item, err := q.scanRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func (q *QueryBuilder[T]) One(args ...any) (T, error) {
	q.addWhere(args...)
	var zero T
	query, args := q.buildQuery()
	row := q.store.DB.QueryRow(query, args...)
	item, err := q.scanRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return zero, fmt.Errorf("no %s found", q.type_.Name())
		}
		return zero, err
	}
	return item, nil
}

func (q *QueryBuilder[T]) First(args ...any) (T, error) {
	q.addWhere(args...)
	return q.OrderBy("rowid ASC").One()
}

func (q *QueryBuilder[T]) Last(args ...any) (T, error) {
	q.addWhere(args...)
	return q.OrderBy("rowid DESC").One()
}

func (q *QueryBuilder[T]) Count(args ...any) (int, error) {
	q.addWhere(args...)
	query, args := q.buildQuery()
	query = strings.Replace(query, "SELECT *", "SELECT COUNT(*)", 1)
	var count int
	err := q.store.DB.QueryRow(query, args...).Scan(&count)
	return count, err
}

func (q *QueryBuilder[T]) SQL() string {
	query, _ := q.buildQuery()

	return query
}
