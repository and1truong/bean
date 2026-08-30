package dbal

import (
	"context"
	"errors"
	"fmt"
)

type ErrorCode string

const (
	Conflict             ErrorCode = "conflict"
	UniqueViolation      ErrorCode = "unique_violation"
	ForeignKeyViolation  ErrorCode = "foreign_key_violation"
	NotFound             ErrorCode = "not_found"
	Busy                 ErrorCode = "busy"
	SerializationFailure ErrorCode = "serialization_failure"
	Unavailable          ErrorCode = "unavailable"
	InvalidQuery         ErrorCode = "invalid_query"
	Internal             ErrorCode = "internal"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func (e *Error) Unwrap() error { return e.Cause }
func IsCode(err error, code ErrorCode) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

type Value any
type Row map[string]Value

type Operator string

const (
	OpEQ         Operator = "eq"
	OpNE         Operator = "ne"
	OpGT         Operator = "gt"
	OpGTE        Operator = "gte"
	OpLT         Operator = "lt"
	OpLTE        Operator = "lte"
	OpIn         Operator = "in"
	OpNotIn      Operator = "not_in"
	OpIsNull     Operator = "is_null"
	OpIsNotNull  Operator = "is_not_null"
	OpContains   Operator = "contains"
	OpStartsWith Operator = "starts_with"
	OpEndsWith   Operator = "ends_with"
)

type Predicate struct {
	Op       Operator
	Column   string
	Value    Value
	Children []Predicate
}

func And(ps ...Predicate) Predicate { return Predicate{Op: "and", Children: ps} }
func Or(ps ...Predicate) Predicate  { return Predicate{Op: "or", Children: ps} }
func Not(p Predicate) Predicate     { return Predicate{Op: "not", Children: []Predicate{p}} }

type Order struct {
	Column string
	Desc   bool
}
type Join struct{ Table, Alias, Left, Right, Type string }
type Aggregate struct{ Function, Column, Alias string }

type Select struct {
	Table         string
	Columns       []string
	Joins         []Join
	Where         *Predicate
	GroupBy       []string
	Aggregates    []Aggregate
	OrderBy       []Order
	Limit, Offset int
}
type Insert struct {
	Table  string
	Values map[string]Value
}
type Update struct {
	Table        string
	Values       map[string]Value
	Where        Predicate
	ExpectedRows int64
}
type Delete struct {
	Table        string
	Where        Predicate
	ExpectedRows int64
}

type Result struct{ Affected int64 }
type Database interface {
	Select(context.Context, Select) ([]Row, error)
	Insert(context.Context, Insert) (Result, error)
	Update(context.Context, Update) (Result, error)
	Delete(context.Context, Delete) (Result, error)
	Transaction(context.Context, func(Transaction) error) error
	Close() error
}
type Transaction interface {
	Select(context.Context, Select) ([]Row, error)
	Insert(context.Context, Insert) (Result, error)
	Update(context.Context, Update) (Result, error)
	Delete(context.Context, Delete) (Result, error)
}
type Dialect interface {
	QuoteIdentifier(string) (string, error)
	Placeholder(int) string
}
type QueryCompiler interface {
	CompileSelect(Select) (string, []Value, error)
}
type SchemaInspector interface {
	Tables(context.Context) ([]string, error)
	Columns(context.Context, string) ([]Column, error)
}
type Column struct {
	Name, LogicalType string
	Nullable          bool
}
type MigrationExecutor interface {
	Apply(context.Context, []Migration) error
}
type Migration struct {
	ID, Description string
	Statements      []string
}
