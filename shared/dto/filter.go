// Package dto provides data transfer objects for the application.
package dto

import (
	"fmt"
	"maps"
	"reflect"
	"strings"
)

const (
	// FilterOperatorEq is used to indicate that the filter should check for equality between the field and the value (e.g., field = value)
	FilterOperatorEq = "eq"
	// FilterOperatorLike is used to indicate that the filter should check for a pattern match using the SQL LIKE operator (e.g., field LIKE '%value%')
	FilterOperatorLike = "like"
	// FilterOperatorIn is used to indicate that the filter should check if the field's value is within a specified set of values using the SQL IN operator (e.g., field IN (value1, value2, ...))
	FilterOperatorIn = "in"
	// FilterOperatorNotEq is used to indicate that the filter should check for inequality between the field and the value (e.g., field != value)
	FilterOperatorNotEq = "not_eq"
	// FilterOperatorLessEq is used to indicate that the filter should check if the field's value is less than or equal to the specified value (e.g., field <= value)
	FilterOperatorLessEq = "less_eq"
	// FilterOperatorLessThan is used to indicate that the filter should check if the field's value is less than the specified value (e.g., field < value)
	FilterOperatorLessThan = "less_than"
	// FilterOperatorGreaterEq is used to indicate that the filter should check if the field's value is greater than or equal to the specified value (e.g., field >= value)
	FilterOperatorGreaterEq = "greater_eq"
	// FilterOperatorGreaterThan is used to indicate that the filter should check if the field's value is greater than the specified value (e.g., field > value)
	FilterOperatorGreaterThan = "greater_than"
	// FilterPlainQuery is used to indicate that the filter value is a raw SQL query that should be included directly in the WHERE clause without any modification or parameterization (e.g., (field1 = value1 AND field2 > value2))
	FilterPlainQuery = "plan"
	// FilterIsNotNull is used to indicate that the filter should check if the field's value is not null (e.g., field IS NOT NULL)
	FilterIsNotNull = "is_not_null"
	// FilterIsNull is used to indicate that the filter should check if the field's value is null (e.g., field IS NULL)
	FilterIsNull = "is_null"
)

const (
	// FilterGroupOperatorAnd is used to indicate that filters in a FilterGroup should be combined with a logical AND
	FilterGroupOperatorAnd = "AND"
	// FilterGroupOperatorOr is used to indicate that filters in a FilterGroup should be combined with a logical OR
	FilterGroupOperatorOr = "OR"
)

// Filter represents a single filter condition for querying data.
// It includes the field to filter on, the operator to use, and the value to compare against.
// The GetWhereClause method generates the corresponding SQL WHERE clause and a map of named parameters for query execution.
type Filter struct {
	ArgName  string
	Field    string
	Value    any
	Operator string `validate:"required,oneof=eq like in not_eq less_eq greater_eq"`
	Table    string
}

// GetWhereClause generates the SQL WHERE clause for the filter based on its operator and value,
// and returns the clause along with a map of named parameters for query execution.
func (f *Filter) GetWhereClause() (string, map[string]any) {
	args := map[string]any{}

	column := f.Field
	if f.Table != "" {
		column = fmt.Sprintf("%s.%s", f.Table, f.Field)
	}

	argName := f.ArgName
	if argName == "" {
		argName = f.Field
	}

	switch f.Operator {
	case FilterOperatorEq:
		args[argName] = f.Value

		return fmt.Sprintf("%s = :%s", column, argName), args
	case FilterOperatorLike:
		args[argName] = fmt.Sprintf("%%%s%%", f.Value)

		return fmt.Sprintf("LOWER(%s) LIKE LOWER(:%s) ", column, argName), args
	case FilterOperatorIn:
		val := reflect.ValueOf(f.Value)
		vType := val.Type()

		switch vType.Kind() {
		case reflect.Array, reflect.Slice:
			named := make([]string, val.Len())

			for idx := range val.Len() {
				args[fmt.Sprintf("%s_%d", argName, idx)] = val.Index(idx).Interface()

				named[idx] = fmt.Sprintf(":%s_%d", argName, idx)
			}

			return fmt.Sprintf("%s IN (%s) ", column, strings.Join(named, ", ")), args
		default:
			return fmt.Sprintf("%s IN (%s) ", column, f.Value), args
		}
	case FilterOperatorNotEq:
		args[argName] = f.Value

		return fmt.Sprintf("%s != :%s", column, argName), args
	case FilterOperatorLessEq:
		args[argName] = f.Value

		return fmt.Sprintf("%s <= :%s", column, argName), args
	case FilterOperatorGreaterEq:
		args[argName] = f.Value

		return fmt.Sprintf("%s >= :%s", column, argName), args
	case FilterPlainQuery:
		query, _ := f.Value.(string)

		return fmt.Sprintf("(%s)", query), args
	case FilterIsNotNull:
		return column + " IS NOT NULL", args
	case FilterIsNull:
		return column + " IS NULL", args
	default:
		return "", args
	}
}

// FilterGroup represents a group of filters combined with a logical operator (AND/OR). It can contain both individual filters and nested filter groups, allowing for complex query conditions.
type FilterGroup struct {
	Filters  []any
	Operator string
}

// GetWhereClause generates the combined WHERE clause for the filter group, including all nested filters and groups, and returns the clause along with a map of named parameters for query execution.
func (f *FilterGroup) GetWhereClause() (string, map[string]any) {
	args := map[string]any{}
	whereClause := []string{}

	for _, filter := range f.Filters {
		switch fill := filter.(type) {
		case Filter:
			where, arg := fill.GetWhereClause()
			whereClause = append(whereClause, where)

			maps.Copy(args, arg)
		case FilterGroup:
			where, arg := fill.GetWhereClause()
			whereClause = append(whereClause, where)

			maps.Copy(args, arg)
		}
	}

	if len(whereClause) == 0 {
		return "", args
	}

	return fmt.Sprintf("(%s)", strings.Join(whereClause, " "+f.Operator+" ")), args
}
