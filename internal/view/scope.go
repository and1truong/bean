package view

import (
	"context"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/policy"
)

// ResolvedRecord is an immutable, View-projected record snapshot.
type ResolvedRecord struct {
	Entity string
	View   string
	ID     string
	row    dbal.Row
}

// Row returns a copy so consumers cannot mutate a cached snapshot.
func (r ResolvedRecord) Row() dbal.Row { return cloneRow(r.row) }

// Scope binds resolved records and authorization proofs to one request, AppIR,
// reader, and complete authorization context.
type Scope struct {
	app     *appir.App
	reader  Reader
	request beanctx.Request
	records map[string]ResolvedRecord
	proofs  map[string]bool
}

func NewScope(app *appir.App, reader Reader, request beanctx.Request) *Scope {
	return &Scope{app: app, reader: reader, request: cloneRequest(request), records: map[string]ResolvedRecord{}, proofs: map[string]bool{}}
}

func (s *Scope) App() *appir.App          { return s.app }
func (s *Scope) Reader() Reader           { return s.reader }
func (s *Scope) Request() beanctx.Request { return cloneRequest(s.request) }

func (s *Scope) Resolve(ctx context.Context, viewName, id string) (ResolvedRecord, error) {
	key := viewName + "\x00" + id
	if record, ok := s.records[key]; ok {
		return ResolvedRecord{Entity: record.Entity, View: record.View, ID: record.ID, row: cloneRow(record.row)}, nil
	}
	definition, ok := s.app.Views[viewName]
	if !ok {
		return ResolvedRecord{}, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
	}
	result, err := ReadPage(ctx, s.reader, s.app, viewName, ReadOptions{Params: Params{RecordID: id, Limit: 1}}, s.request)
	if err != nil {
		return ResolvedRecord{}, err
	}
	if len(result.Rows) == 0 {
		return ResolvedRecord{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	record := ResolvedRecord{Entity: definition.Entity, View: viewName, ID: id, row: cloneRow(result.Rows[0])}
	s.records[key] = record
	if entityProofCompatible(s.app, definition) {
		s.proofs[definition.Entity+"\x00"+id] = true
	}
	return ResolvedRecord{Entity: record.Entity, View: record.View, ID: record.ID, row: cloneRow(record.row)}, nil
}

// AuthorizeEntity proves that the request can read one Entity record. Compatible
// View resolutions satisfy this check without exposing their projection as a
// full Entity record.
func (s *Scope) AuthorizeEntity(ctx context.Context, entity, id string) error {
	key := entity + "\x00" + id
	if s.proofs[key] {
		return nil
	}
	if _, err := ReadEntityRecord(ctx, s.reader, s.app, entity, id, s.request); err != nil {
		return err
	}
	s.proofs[key] = true
	return nil
}

func entityProofCompatible(app *appir.App, view appir.View) bool {
	entity, ok := app.Entities[view.Entity]
	return ok && policy.EffectiveViewPolicyName(view, entity) == entity.Policy && view.Filter == nil && view.ContextFilter == nil && len(view.Relationships) == 0 && len(view.GroupBy) == 0 && len(view.Aggregates) == 0
}

func cloneRow(row dbal.Row) dbal.Row {
	copy := make(dbal.Row, len(row))
	for key, value := range row {
		copy[key] = cloneValue(value)
	}
	return copy
}

func cloneRequest(request beanctx.Request) beanctx.Request {
	copy := request
	if request.User != nil {
		user := *request.User
		user.Roles = append([]string(nil), request.User.Roles...)
		copy.User = &user
	}
	copy.RouteParams = cloneStrings(request.RouteParams)
	copy.Entity = cloneValues(request.Entity)
	copy.Values = cloneValues(request.Values)
	return copy
}

func cloneStrings(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

func cloneValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	copy := make(map[string]any, len(values))
	for key, value := range values {
		copy[key] = cloneValue(value)
	}
	return copy
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneValues(typed)
	case dbal.Row:
		return cloneRow(typed)
	case []any:
		copy := make([]any, len(typed))
		for index, item := range typed {
			copy[index] = cloneValue(item)
		}
		return copy
	case []string:
		return append([]string(nil), typed...)
	case []byte:
		return append([]byte(nil), typed...)
	default:
		return value
	}
}
