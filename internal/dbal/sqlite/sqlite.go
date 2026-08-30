package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/beanruntime/bean/internal/dbal"
	_ "modernc.org/sqlite"
)

type DB struct {
	db       *sql.DB
	compiler Compiler
}

func Open(path string) (*DB, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	dsn := path + separator + "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(8)
	d.SetMaxIdleConns(2)
	if err = d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	return &DB{db: d}, nil
}
func (d *DB) Close() error { return d.db.Close() }
func (d *DB) Select(ctx context.Context, q dbal.Select) ([]dbal.Row, error) {
	return selectRows(ctx, d.db, d.compiler, q)
}
func (d *DB) Insert(ctx context.Context, q dbal.Insert) (dbal.Result, error) {
	return insert(ctx, d.db, d.compiler, q)
}
func (d *DB) Update(ctx context.Context, q dbal.Update) (dbal.Result, error) {
	return update(ctx, d.db, d.compiler, q)
}
func (d *DB) Delete(ctx context.Context, q dbal.Delete) (dbal.Result, error) {
	return deleteRows(ctx, d.db, d.compiler, q)
}
func (d *DB) Transaction(ctx context.Context, fn func(dbal.Transaction) error) error {
	tx, e := d.db.BeginTx(ctx, nil)
	if e != nil {
		return translate(e)
	}
	w := &transaction{tx: tx, compiler: d.compiler}
	if e = fn(w); e != nil {
		_ = tx.Rollback()
		return e
	}
	return translate(tx.Commit())
}

type transaction struct {
	tx       *sql.Tx
	compiler Compiler
}

func (t *transaction) Select(c context.Context, q dbal.Select) ([]dbal.Row, error) {
	return selectRows(c, t.tx, t.compiler, q)
}
func (t *transaction) Insert(c context.Context, q dbal.Insert) (dbal.Result, error) {
	return insert(c, t.tx, t.compiler, q)
}
func (t *transaction) Update(c context.Context, q dbal.Update) (dbal.Result, error) {
	return update(c, t.tx, t.compiler, q)
}
func (t *transaction) Delete(c context.Context, q dbal.Delete) (dbal.Result, error) {
	return deleteRows(c, t.tx, t.compiler, q)
}

type runner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func selectRows(ctx context.Context, r runner, c Compiler, q dbal.Select) ([]dbal.Row, error) {
	s, args, e := c.CompileSelect(q)
	if e != nil {
		return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: e.Error(), Cause: e}
	}
	rows, e := r.QueryContext(ctx, s, anyValues(args)...)
	if e != nil {
		return nil, translate(e)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []dbal.Row{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if e = rows.Scan(ptrs...); e != nil {
			return nil, translate(e)
		}
		row := dbal.Row{}
		for i, k := range cols {
			row[k] = vals[i]
		}
		out = append(out, row)
	}
	return out, translate(rows.Err())
}
func insert(ctx context.Context, r runner, c Compiler, q dbal.Insert) (dbal.Result, error) {
	if len(q.Values) == 0 {
		return dbal.Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "empty insert"}
	}
	table, e := c.QuoteIdentifier(q.Table)
	if e != nil {
		return dbal.Result{}, bad(e)
	}
	keys := make([]string, 0, len(q.Values))
	for k := range q.Values {
		keys = append(keys, k)
	}
	sortStrings(keys)
	cols := []string{}
	marks := []string{}
	args := []any{}
	for _, k := range keys {
		x, e := c.QuoteIdentifier(k)
		if e != nil {
			return dbal.Result{}, bad(e)
		}
		cols = append(cols, x)
		marks = append(marks, "?")
		args = append(args, q.Values[k])
	}
	res, e := r.ExecContext(ctx, "INSERT INTO "+table+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", args...)
	return result(res, e, 0)
}
func update(ctx context.Context, r runner, c Compiler, q dbal.Update) (dbal.Result, error) {
	table, e := c.QuoteIdentifier(q.Table)
	if e != nil {
		return dbal.Result{}, bad(e)
	}
	keys := make([]string, 0, len(q.Values))
	for k := range q.Values {
		keys = append(keys, k)
	}
	sortStrings(keys)
	sets := []string{}
	args := []dbal.Value{}
	for _, k := range keys {
		x, e := c.QuoteIdentifier(k)
		if e != nil {
			return dbal.Result{}, bad(e)
		}
		sets = append(sets, x+" = ?")
		args = append(args, q.Values[k])
	}
	w, e := c.predicate(q.Where, &args)
	if e != nil {
		return dbal.Result{}, bad(e)
	}
	res, e := r.ExecContext(ctx, "UPDATE "+table+" SET "+strings.Join(sets, ",")+" WHERE "+w, anyValues(args)...)
	return result(res, e, q.ExpectedRows)
}
func deleteRows(ctx context.Context, r runner, c Compiler, q dbal.Delete) (dbal.Result, error) {
	table, e := c.QuoteIdentifier(q.Table)
	if e != nil {
		return dbal.Result{}, bad(e)
	}
	args := []dbal.Value{}
	w, e := c.predicate(q.Where, &args)
	if e != nil {
		return dbal.Result{}, bad(e)
	}
	res, e := r.ExecContext(ctx, "DELETE FROM "+table+" WHERE "+w, anyValues(args)...)
	return result(res, e, q.ExpectedRows)
}
func result(r sql.Result, e error, expected int64) (dbal.Result, error) {
	if e != nil {
		return dbal.Result{}, translate(e)
	}
	n, e := r.RowsAffected()
	if e != nil {
		return dbal.Result{}, translate(e)
	}
	if expected > 0 && n != expected {
		return dbal.Result{}, &dbal.Error{Code: dbal.Conflict, Message: "affected-row check failed"}
	}
	return dbal.Result{Affected: n}, nil
}
func bad(e error) error { return &dbal.Error{Code: dbal.InvalidQuery, Message: e.Error(), Cause: e} }
func translate(e error) error {
	if e == nil {
		return nil
	}
	s := strings.ToLower(e.Error())
	code := dbal.Internal
	switch {
	case strings.Contains(s, "unique constraint"):
		code = dbal.UniqueViolation
	case strings.Contains(s, "foreign key constraint"):
		code = dbal.ForeignKeyViolation
	case strings.Contains(s, "locked") || strings.Contains(s, "busy"):
		code = dbal.Busy
	case errors.Is(e, sql.ErrNoRows):
		code = dbal.NotFound
	}
	return &dbal.Error{Code: code, Message: "database operation failed", Cause: e}
}
func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
func anyValues(values []dbal.Value) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}
func (d *DB) Tables(ctx context.Context) ([]string, error) {
	rows, e := d.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if e != nil {
		return nil, translate(e)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if e = rows.Scan(&s); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (d *DB) Columns(ctx context.Context, table string) ([]dbal.Column, error) {
	q, e := d.compiler.QuoteIdentifier(table)
	if e != nil {
		return nil, bad(e)
	}
	rows, e := d.db.QueryContext(ctx, "PRAGMA table_info("+q+")")
	if e != nil {
		return nil, translate(e)
	}
	defer rows.Close()
	out := []dbal.Column{}
	for rows.Next() {
		var seq, notnull, pk int
		var name, typ string
		var def any
		if e = rows.Scan(&seq, &name, &typ, &notnull, &def, &pk); e != nil {
			return nil, e
		}
		out = append(out, dbal.Column{Name: name, LogicalType: strings.ToLower(typ), Nullable: notnull == 0})
	}
	return out, rows.Err()
}
func (d *DB) ExecuteMigration(ctx context.Context, statements []string) error {
	tx, e := d.db.BeginTx(ctx, nil)
	if e != nil {
		return translate(e)
	}
	for _, s := range statements {
		if _, e = tx.ExecContext(ctx, s); e != nil {
			_ = tx.Rollback()
			return translate(e)
		}
	}
	return translate(tx.Commit())
}

var _ dbal.Database = (*DB)(nil)
var _ dbal.SchemaInspector = (*DB)(nil)
