package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	db       *sql.DB
	compiler Compiler
}

func Open(databaseURL string) (*DB, error) {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(16)
	database.SetMaxIdleConns(4)
	if err = database.Ping(); err != nil {
		database.Close()
		return nil, translate(err)
	}
	return &DB{db: database}, nil
}

func (d *DB) Close() error    { return d.db.Close() }
func (d *DB) Dialect() string { return "postgres" }
func (d *DB) WithPublicationLock(ctx context.Context, appID string, operation func() error) error {
	connection, err := d.db.Conn(ctx)
	if err != nil {
		return translate(err)
	}
	defer connection.Close()
	key := "bean-publication/" + appID
	if _, err = connection.ExecContext(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, key); err != nil {
		return translate(err)
	}
	defer connection.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, key)
	return operation()
}
func (d *DB) Select(ctx context.Context, query dbal.Select) ([]dbal.Row, error) {
	return selectRows(ctx, d.db, d.compiler, query)
}
func (d *DB) Insert(ctx context.Context, query dbal.Insert) (dbal.Result, error) {
	return insert(ctx, d.db, d.compiler, query)
}
func (d *DB) Update(ctx context.Context, query dbal.Update) (dbal.Result, error) {
	return update(ctx, d.db, d.compiler, query)
}
func (d *DB) Delete(ctx context.Context, query dbal.Delete) (dbal.Result, error) {
	return deleteRows(ctx, d.db, d.compiler, query)
}
func (d *DB) Transaction(ctx context.Context, operation func(dbal.Transaction) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = d.transaction(ctx, operation)
		if !dbal.IsCode(err, dbal.SerializationFailure) {
			return err
		}
	}
	return err
}

func (d *DB) transaction(ctx context.Context, operation func(dbal.Transaction) error) error {
	transaction, err := d.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return translate(err)
	}
	wrapper := &tx{tx: transaction, compiler: d.compiler}
	if err = operation(wrapper); err != nil {
		_ = transaction.Rollback()
		return err
	}
	return translate(transaction.Commit())
}

type tx struct {
	tx       *sql.Tx
	compiler Compiler
}

func (t *tx) Select(ctx context.Context, query dbal.Select) ([]dbal.Row, error) {
	return selectRows(ctx, t.tx, t.compiler, query)
}
func (t *tx) Insert(ctx context.Context, query dbal.Insert) (dbal.Result, error) {
	return insert(ctx, t.tx, t.compiler, query)
}
func (t *tx) Update(ctx context.Context, query dbal.Update) (dbal.Result, error) {
	return update(ctx, t.tx, t.compiler, query)
}
func (t *tx) Delete(ctx context.Context, query dbal.Delete) (dbal.Result, error) {
	return deleteRows(ctx, t.tx, t.compiler, query)
}

type runner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func selectRows(ctx context.Context, runner runner, compiler Compiler, query dbal.Select) ([]dbal.Row, error) {
	statement, arguments, err := compiler.CompileSelect(query)
	if err != nil {
		return nil, invalid(err)
	}
	rows, err := runner.QueryContext(ctx, statement, values(arguments)...)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	columns, _ := rows.Columns()
	result := []dbal.Row{}
	for rows.Next() {
		scanned := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for index := range scanned {
			pointers[index] = &scanned[index]
		}
		if err = rows.Scan(pointers...); err != nil {
			return nil, translate(err)
		}
		row := dbal.Row{}
		for index, name := range columns {
			row[name] = scanned[index]
		}
		result = append(result, row)
	}
	return result, translate(rows.Err())
}

func insert(ctx context.Context, runner runner, compiler Compiler, query dbal.Insert) (dbal.Result, error) {
	if len(query.Values) == 0 {
		return dbal.Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "empty insert"}
	}
	table, err := compiler.QuoteIdentifier(query.Table)
	if err != nil {
		return dbal.Result{}, invalid(err)
	}
	keys := sortedKeys(query.Values)
	columns := make([]string, 0, len(keys))
	marks := make([]string, 0, len(keys))
	arguments := make([]any, 0, len(keys))
	for index, key := range keys {
		column, quoteErr := compiler.QuoteIdentifier(key)
		if quoteErr != nil {
			return dbal.Result{}, invalid(quoteErr)
		}
		columns = append(columns, column)
		marks = append(marks, compiler.Placeholder(index+1))
		arguments = append(arguments, query.Values[key])
	}
	result, err := runner.ExecContext(ctx, "INSERT INTO "+table+" ("+strings.Join(columns, ",")+") VALUES ("+strings.Join(marks, ",")+")", arguments...)
	return checkedResult(result, err, 0)
}

func update(ctx context.Context, runner runner, compiler Compiler, query dbal.Update) (dbal.Result, error) {
	table, err := compiler.QuoteIdentifier(query.Table)
	if err != nil {
		return dbal.Result{}, invalid(err)
	}
	keys := sortedKeys(query.Values)
	sets := make([]string, 0, len(keys))
	arguments := make([]dbal.Value, 0, len(keys))
	for _, key := range keys {
		column, quoteErr := compiler.QuoteIdentifier(key)
		if quoteErr != nil {
			return dbal.Result{}, invalid(quoteErr)
		}
		sets = append(sets, column+" = ?")
		arguments = append(arguments, query.Values[key])
	}
	where, predicateArguments, err := compilePredicate(compiler, query.Where)
	if err != nil {
		return dbal.Result{}, invalid(err)
	}
	arguments = append(arguments, predicateArguments...)
	statement := numberPlaceholders("UPDATE " + table + " SET " + strings.Join(sets, ",") + " WHERE " + where)
	result, err := runner.ExecContext(ctx, statement, values(arguments)...)
	return checkedResult(result, err, query.ExpectedRows)
}

func deleteRows(ctx context.Context, runner runner, compiler Compiler, query dbal.Delete) (dbal.Result, error) {
	table, err := compiler.QuoteIdentifier(query.Table)
	if err != nil {
		return dbal.Result{}, invalid(err)
	}
	where, arguments, err := compilePredicate(compiler, query.Where)
	if err != nil {
		return dbal.Result{}, invalid(err)
	}
	result, err := runner.ExecContext(ctx, numberPlaceholders("DELETE FROM "+table+" WHERE "+where), values(arguments)...)
	return checkedResult(result, err, query.ExpectedRows)
}

func compilePredicate(compiler Compiler, predicate dbal.Predicate) (string, []dbal.Value, error) {
	statement, arguments, err := compiler.CompileSelect(dbal.Select{Table: "bean_predicate", Where: &predicate})
	if err != nil {
		return "", nil, err
	}
	prefix := `SELECT * FROM "bean_predicate" WHERE `
	statement = strings.TrimPrefix(statement, prefix)
	for index := range arguments {
		statement = strings.Replace(statement, compiler.Placeholder(index+1), "?", 1)
	}
	return statement, arguments, nil
}

func checkedResult(result sql.Result, err error, expected int64) (dbal.Result, error) {
	if err != nil {
		return dbal.Result{}, translate(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return dbal.Result{}, translate(err)
	}
	if expected > 0 && affected != expected {
		return dbal.Result{}, &dbal.Error{Code: dbal.Conflict, Message: "affected-row check failed"}
	}
	return dbal.Result{Affected: affected}, nil
}

func (d *DB) Tables(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE' ORDER BY table_name`)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	tables := []string{}
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			return nil, translate(err)
		}
		tables = append(tables, table)
	}
	return tables, translate(rows.Err())
}

func (d *DB) Columns(ctx context.Context, table string) ([]dbal.Column, error) {
	if _, err := d.compiler.QuoteIdentifier(table); err != nil {
		return nil, invalid(err)
	}
	rows, err := d.db.QueryContext(ctx, `SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1 ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	columns := []dbal.Column{}
	for rows.Next() {
		var name, logicalType, nullable string
		if err = rows.Scan(&name, &logicalType, &nullable); err != nil {
			return nil, translate(err)
		}
		columns = append(columns, dbal.Column{Name: name, LogicalType: logicalType, Nullable: nullable == "YES"})
	}
	return columns, translate(rows.Err())
}

func (d *DB) ExecuteMigration(ctx context.Context, statements []string) error {
	transaction, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return translate(err)
	}
	for _, statement := range statements {
		if _, err = transaction.ExecContext(ctx, statement); err != nil {
			_ = transaction.Rollback()
			return translate(err)
		}
	}
	return translate(transaction.Commit())
}

func invalid(err error) error {
	return &dbal.Error{Code: dbal.InvalidQuery, Message: err.Error(), Cause: err}
}

func translate(err error) error {
	if err == nil {
		return nil
	}
	code := dbal.Internal
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			code = dbal.UniqueViolation
		case "23503":
			code = dbal.ForeignKeyViolation
		case "40001", "40P01":
			code = dbal.SerializationFailure
		case "08000", "08001", "08003", "08004", "08006", "08007", "08P01", "57P01", "57P02", "57P03":
			code = dbal.Unavailable
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		code = dbal.NotFound
	}
	return &dbal.Error{Code: code, Message: "database operation failed", Cause: err}
}

var _ dbal.PublicationLocker = (*DB)(nil)

func sortedKeys(input map[string]dbal.Value) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	for index := 1; index < len(keys); index++ {
		for current := index; current > 0 && keys[current] < keys[current-1]; current-- {
			keys[current], keys[current-1] = keys[current-1], keys[current]
		}
	}
	return keys
}

func values(input []dbal.Value) []any {
	output := make([]any, len(input))
	for index, value := range input {
		output[index] = value
	}
	return output
}

var _ dbal.Database = (*DB)(nil)
var _ dbal.SchemaInspector = (*DB)(nil)
