// Package repository provides generic repository implementations for database operations.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go-espn-api/infras/otel"
	"go-espn-api/infras/postgres"
	"go-espn-api/shared/constant"
	"go-espn-api/shared/dto"
	"go-espn-api/shared/logger"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/jmoiron/sqlx"
)

var (
	errRequiredFilter = errors.New("required filter")
)

type column struct {
	name  string
	table string
	alias string
}

type execer interface {
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
}

// Repository is a generic repository struct that provides common database operations for any entity type T.
type Repository[T any] struct {
	db            *postgres.Connection
	otel          otel.Otel
	table         string
	entity        string
	primaryColumn string
	columns       []column
	join          string
	InsertColumns []string
}

// NewRepository creates a new instance of Repository for a specific entity type T,
// initializing it with the necessary database connection, table name, primary column, and OpenTelemetry instance for tracing.
// It uses reflection to determine the columns of the entity and any join queries defined by the entity's methods.
func NewRepository[T any](entityName, tableName, primaryColumn string, dbConnection *postgres.Connection, otl otel.Otel) Repository[T] {
	var zero T

	reflectType := reflect.TypeOf(zero)
	columns, insertColumns := getColumns(tableName, reflectType)

	valueOf := reflect.ValueOf(zero)
	method := valueOf.MethodByName("GetJoinQuery")
	joinQueryStr := ""

	if method.IsValid() {
		joinQuery := method.Call([]reflect.Value{})

		if len(joinQuery) > 0 {
			joinQueryStr = joinQuery[0].String()
		}
	}

	return Repository[T]{
		db:            dbConnection,
		otel:          otl,
		table:         tableName,
		entity:        entityName,
		primaryColumn: primaryColumn,
		columns:       columns,
		join:          joinQueryStr,
		InsertColumns: insertColumns,
	}
}

// insert is a helper method that performs the actual insertion of a record into the database,
// abstracting the common logic for both Insert and InsertTx methods. It takes a context,
// an execer (which can be either a sqlx.DB or sqlx.Tx), and the model to be inserted, returning an error if the operation fails.
func (repo *Repository[T]) insert(ctx context.Context, exec execer, model T) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.insert", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	placeholders := make([]string, 0, len(repo.InsertColumns))

	for _, col := range repo.InsertColumns {
		placeholders = append(placeholders, ":"+col)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", repo.table, strings.Join(repo.InsertColumns, ", "), strings.Join(placeholders, ", "))
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	_, err := exec.NamedExecContext(ctx, query, model)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return fmt.Errorf("failed to insert data (%s): %w", repo.entity, err)
	}

	return nil
}

// Insert inserts a new record into the database, returning an error if the operation fails.
func (repo *Repository[T]) Insert(ctx context.Context, model T) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.Insert", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	return repo.insert(ctx, repo.db.Write, model) //nolint:wrapcheck
}

// InsertTx inserts a new record into the database within the context of an existing transaction,
// allowing for atomic operations and ensuring that the insert operation is part of the transaction's scope.
// It takes a context, a sqlx.Tx transaction object, and the model to be inserted, returning an error if the operation fails.
func (repo *Repository[T]) InsertTx(ctx context.Context, sqltx *sqlx.Tx, model T) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.InsertTx", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	return repo.insert(ctx, sqltx, model) //nolint:wrapcheck
}

// Exist checks if a record exists that matches the given filter criteria, returning true if it exists and false otherwise, along with any error encountered during the operation.
func (repo *Repository[T]) Exist(ctx context.Context, filter dto.FilterGroup) (bool, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.Exist", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	where, args := repo.BuildWhereClause(ctx, filter)
	if where == "" {
		return false, errRequiredFilter
	}

	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s %s)", repo.table, where)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	exist := false

	prepare, err := repo.db.Read.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return false, fmt.Errorf("failed to check exist data (%s): %w", repo.entity, err)
	}
	defer func(prepare *sqlx.NamedStmt) {
		err := prepare.Close()
		if err != nil {
			logger.ErrorWithStack(err)
		}
	}(prepare)

	err = prepare.GetContext(ctx, &exist, args)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return false, fmt.Errorf("failed to check exist data (%s): %w", repo.entity, err)
	}

	return exist, nil
}

// Get returns a single record that matches the given filter criteria, along with any error encountered during the operation.
func (repo *Repository[T]) Get(ctx context.Context, filter dto.FilterGroup, columns ...string) (T, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.Get", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	where, args := repo.BuildWhereClause(ctx, filter)
	selectQuery := repo.getSelectQuery(ctx, columns...)

	query := fmt.Sprintf("SELECT %s FROM %s %s %s", selectQuery, repo.table, repo.join, where)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var model T

	prepare, err := repo.db.Read.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return model, fmt.Errorf("failed to prepare statement (%s): %w", repo.entity, err)
	}
	defer func(prepare *sqlx.NamedStmt) {
		err := prepare.Close()
		if err != nil {
			logger.ErrorWithStack(err)
		}
	}(prepare)

	err = prepare.GetContext(ctx, &model, args)
	if errors.Is(err, sql.ErrNoRows) {
		return model, nil
	}

	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return model, fmt.Errorf("failed to get data (%s): %w", repo.entity, err)
	}

	return model, nil
}

// GetAll returns a slice of records that match the given filter criteria, along with any error encountered during the operation.
func (repo *Repository[T]) GetAll(ctx context.Context, params dto.QueryParams, filter dto.FilterGroup, columns ...string) ([]T, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.GetAll", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	where, args := repo.BuildWhereClause(ctx, filter)
	selectQuery := repo.getSelectQuery(ctx, columns...)

	var ordering, pagination string

	page := params.Page
	limit := params.Limit

	if page > 0 && limit > 0 {
		args["limit"] = limit
		args["offset"] = (page - 1) * limit

		pagination = "LIMIT :limit OFFSET :offset"
	} else if limit > 0 {
		args["limit"] = limit

		pagination = "LIMIT :limit"
	}

	if params.SortBy != "" && params.SortDir != "" {
		ordering = fmt.Sprintf("ORDER BY %s %s", params.SortBy, params.SortDir)
	}

	query := fmt.Sprintf("SELECT %s FROM %s %s %s %s %s", selectQuery, repo.table, repo.join, where, ordering, pagination)

	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var models []T

	prepare, err := repo.db.Read.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return models, fmt.Errorf("failed to prepare statement (%s): %w", repo.entity, err)
	}
	defer func(prepare *sqlx.NamedStmt) {
		err := prepare.Close()
		if err != nil {
			logger.ErrorWithStack(err)
		}
	}(prepare)

	err = prepare.SelectContext(ctx, &models, args)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return models, fmt.Errorf("failed to get all data (%s): %w", repo.entity, err)
	}

	return models, nil
}

// Count returns the total number of records that match the given filter criteria.
func (repo *Repository[T]) Count(ctx context.Context, filter dto.FilterGroup) (int, error) {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.Count", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	where, args := repo.BuildWhereClause(ctx, filter)

	query := fmt.Sprintf("SELECT COUNT(%s.%s) FROM %s %s %s", repo.table, repo.primaryColumn, repo.table, repo.join, where)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	var count int

	prepare, err := repo.db.Read.PrepareNamedContext(ctx, query)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return 0, fmt.Errorf("failed to prepare statement (%s): %w", repo.entity, err)
	}
	defer func(prepare *sqlx.NamedStmt) {
		err := prepare.Close()
		if err != nil {
			logger.ErrorWithStack(err)
		}
	}(prepare)

	err = prepare.GetContext(ctx, &count, args)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return 0, fmt.Errorf("failed to count data (%s): %w", repo.entity, err)
	}

	return count, nil
}

func (repo *Repository[T]) delete(ctx context.Context, exec execer, filter dto.FilterGroup) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.delete", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	where, args := repo.BuildWhereClause(ctx, filter)
	if where == "" {
		return errRequiredFilter
	}

	query := fmt.Sprintf("DELETE FROM %s %s", repo.table, where)
	scope.SetAttribute(constant.OtelQueryAttributeKey, query)

	_, err := exec.NamedExecContext(ctx, query, args)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return fmt.Errorf("failed to delete data (%s): %w", repo.entity, err)
	}

	return nil
}

// Delete removes records matching the filter from the database.
func (repo *Repository[T]) Delete(ctx context.Context, filter dto.FilterGroup) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.Delete", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	return repo.delete(ctx, repo.db.Write, filter) //nolint:wrapcheck
}

// DeleteTx removes records matching the filter within a transaction.
func (repo *Repository[T]) DeleteTx(ctx context.Context, sqltx *sqlx.Tx, filter dto.FilterGroup) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.DeleteTx", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	return repo.delete(ctx, sqltx, filter) //nolint:wrapcheck
}

func (repo *Repository[T]) update(ctx context.Context, exec execer, mod map[string]any, filter dto.FilterGroup) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".update")
	defer scope.End()

	updateField := make([]string, 0, len(mod))

	for col := range maps.Keys(mod) {
		updateField = append(updateField, fmt.Sprintf("%s = :%s", col, col))
	}

	where, args := repo.BuildWhereClause(ctx, filter)
	updateQuery := strings.Join(updateField, ", ")
	query := fmt.Sprintf("UPDATE %s SET %s %s", repo.table, updateQuery, where)

	scope.SetAttribute(constant.OtelQueryAttributeKey, query)
	maps.Copy(args, mod)

	_, err := exec.NamedExecContext(ctx, query, args)
	if err != nil {
		logger.ErrorWithStack(err)
		scope.TraceError(err)

		return fmt.Errorf("failed to update data (%s): %w", repo.entity, err)
	}

	return nil
}

// Update modifies records matching the filter with the provided values.
func (repo *Repository[T]) Update(ctx context.Context, mod map[string]any, filter dto.FilterGroup) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".Update")
	defer scope.End()

	return repo.update(ctx, repo.db.Write, mod, filter) //nolint:wrapcheck
}

// UpdateTx modifies records matching the filter within a transaction.
func (repo *Repository[T]) UpdateTx(ctx context.Context, sqltx *sqlx.Tx, mod map[string]any, filter dto.FilterGroup) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, constant.OtelRepositoryScopeName+".UpdateTx")
	defer scope.End()

	return repo.update(ctx, sqltx, mod, filter) //nolint:wrapcheck
}

// InsertBulk inserts multiple records into the database.
func (repo *Repository[T]) InsertBulk(ctx context.Context, models []T) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.InsertBulk", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	return repo.insertBulk(ctx, repo.db.Write, models)
}

// InsertBulkTx inserts multiple records into the database within a transaction.
func (repo *Repository[T]) InsertBulkTx(ctx context.Context, sqltx *sqlx.Tx, models []T) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.InsertBulkTx", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	return repo.insertBulk(ctx, sqltx, models)
}

func (repo *Repository[T]) insertBulk(ctx context.Context, exec execer, models []T) error {
	ctx, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.insertBulk", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	var err error

	placeholders := make([]string, 0, len(models))

	for _, column := range repo.InsertColumns {
		placeholders = append(placeholders, ":"+column)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", repo.table, strings.Join(repo.InsertColumns, ", "), strings.Join(placeholders, ", "))

	scope.SetAttribute(constant.OtelQueryAttributeKey+repo.entity, query)

	_, err = exec.NamedExecContext(ctx, query, models)
	if err != nil {
		scope.TraceError(err)
		logger.ErrorWithStack(err)

		return fmt.Errorf("failed to bulk insert order: %w", err)
	}

	return nil
}

func (repo *Repository[T]) getSelectQuery(ctx context.Context, columnsParam ...string) string {
	_, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.getSelectQuery", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	columns := []string{}
	for _, col := range repo.columns {
		tableField := col.table
		name := col.name
		alias := col.alias

		if len(columnsParam) > 0 && !slices.Contains(columnsParam, name) {
			continue
		}

		var column string
		if tableField == "" {
			column = name
		} else {
			if alias != "" {
				column = fmt.Sprintf("%s.%s AS %s", tableField, name, alias)
			} else {
				column = fmt.Sprintf("%s.%s", tableField, name)
			}
		}

		columns = append(columns, column)
	}

	return strings.Join(columns, ", ")
}

// BuildWhereClause builds a WHERE clause from the provided filter.
func (repo *Repository[T]) BuildWhereClause(ctx context.Context, filter dto.FilterGroup) (string, map[string]any) {
	_, scope := repo.otel.NewScope(ctx, constant.OtelRepositoryScopeName, fmt.Sprintf("%s.%s.BuildWhereClause", constant.OtelRepositoryScopeName, repo.entity))
	defer scope.End()

	where, args := filter.GetWhereClause()

	if where == "" {
		return where, map[string]any{}
	}

	return fmt.Sprintf(" WHERE %s ", where), args
}

func getColumns(table string, reflectType reflect.Type) (columns []column, insertColumns []string) {
	for i := range reflectType.NumField() {
		field := reflectType.Field(i)
		dbTag := field.Tag.Get("db")
		tableField := field.Tag.Get("table")
		colTag := field.Tag.Get("column")

		if tableField == "" {
			tableField = table
		}

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			col, insertCol := getColumns(table, field.Type)
			columns = append(columns, col...)
			insertColumns = append(insertColumns, insertCol...)
		}

		if dbTag == "" {
			continue
		}

		if tableField == table {
			insertColumns = append(insertColumns, dbTag)
		}

		if colTag == "" {
			columns = append(columns, column{name: dbTag, table: tableField})
		} else {
			columns = append(columns, column{name: colTag, table: tableField, alias: dbTag})
		}
	}

	return columns, insertColumns
}
