package release

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/uid"
)

type Store struct {
	DB         dbal.Database
	Migrations migration.Executor
	Kernel     *kernel.Kernel
	OpenAPI    func(*appir.App) (json.RawMessage, error)
}
type Published struct {
	ID          string `json:"id"`
	Version     int    `json:"version"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	ActivatedAt string `json:"activatedAt"`
}

func (s *Store) Initialize(ctx context.Context) error {
	return s.Migrations.ExecuteMigration(ctx, migration.MetadataSchema())
}
func (s *Store) EnsureApp(ctx context.Context, id, name string) error {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_app", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if e != nil {
		return e
	}
	if len(rows) > 0 {
		return nil
	}
	_, e = s.DB.Insert(ctx, dbal.Insert{Table: "bean_app", Values: map[string]dbal.Value{"id": id, "name": name, "created_at": time.Now().UTC().Format(time.RFC3339Nano)}})
	return e
}
func (s *Store) SaveDefinition(ctx context.Context, appID string, d definition.Definition) error {
	if ds := definition.ValidateEnvelope(d); len(ds) > 0 {
		return ds[0]
	}
	ns := d.Metadata.Namespace
	if ns == "" {
		ns = "default"
	}
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, dbal.Predicate{Op: dbal.OpEQ, Column: "kind", Value: d.Kind}, dbal.Predicate{Op: dbal.OpEQ, Column: "namespace", Value: ns}, dbal.Predicate{Op: dbal.OpEQ, Column: "name", Value: d.Metadata.Name})
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_definition", Where: &where, Limit: 1})
	if e != nil {
		return e
	}
	id := uid.New()
	rev := 1
	if len(rows) > 0 {
		id = fmt.Sprint(rows[0]["id"])
		rev = int(asInt(rows[0]["current_revision"])) + 1
	}
	body, _ := json.Marshal(d)
	sum, _ := definition.Checksum(d)
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		if len(rows) == 0 {
			if _, e := tx.Insert(ctx, dbal.Insert{Table: "bean_definition", Values: map[string]dbal.Value{"id": id, "app_id": appID, "kind": d.Kind, "namespace": ns, "name": d.Metadata.Name, "current_revision": rev}}); e != nil {
				return e
			}
		} else {
			if _, e := tx.Update(ctx, dbal.Update{Table: "bean_definition", Values: map[string]dbal.Value{"current_revision": rev}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, ExpectedRows: 1}); e != nil {
				return e
			}
		}
		_, e := tx.Insert(ctx, dbal.Insert{Table: "bean_definition_revision", Values: map[string]dbal.Value{"definition_id": id, "revision": rev, "checksum": sum, "body": string(body), "created_at": time.Now().UTC().Format(time.RFC3339Nano)}})
		return e
	})
}
func (s *Store) SaveBundle(ctx context.Context, appID string, b definition.Bundle) error {
	if e := s.EnsureApp(ctx, appID, b.Name); e != nil {
		return e
	}
	for _, d := range b.Definitions {
		if e := s.SaveDefinition(ctx, appID, d); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) Draft(ctx context.Context, appID string) ([]definition.Definition, error) {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_definition", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, OrderBy: []dbal.Order{{Column: "kind"}, {Column: "name"}}, Limit: 200})
	if e != nil {
		return nil, e
	}
	out := []definition.Definition{}
	for _, r := range rows {
		p := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "definition_id", Value: r["id"]}, dbal.Predicate{Op: dbal.OpEQ, Column: "revision", Value: r["current_revision"]})
		rr, e := s.DB.Select(ctx, dbal.Select{Table: "bean_definition_revision", Columns: []string{"body"}, Where: &p, Limit: 1})
		if e != nil {
			return nil, e
		}
		if len(rr) != 1 {
			return nil, fmt.Errorf("definition revision is missing")
		}
		var d definition.Definition
		if e = json.Unmarshal([]byte(fmt.Sprint(rr[0]["body"])), &d); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, nil
}
func (s *Store) Validate(ctx context.Context, appID string) (compiler.Result, error) {
	defs, e := s.Draft(ctx, appID)
	if e != nil {
		return compiler.Result{}, e
	}
	return compiler.Compile(appID, s.nextVersion(ctx, appID), defs), nil
}
func (s *Store) Publish(ctx context.Context, appID string) (Published, []definition.Diagnostic, error) {
	r, e := s.Validate(ctx, appID)
	if e != nil {
		return Published{}, nil, e
	}
	if len(r.Diagnostics) > 0 {
		return Published{}, r.Diagnostics, nil
	}
	current, _ := s.activeApp(ctx, appID)
	var old migration.Schema
	if current != nil {
		old = schemaOf(current)
	}
	plan, e := migration.Build(old, r.Schema)
	if e != nil {
		return Published{}, []definition.Diagnostic{{Kind: "Release", Name: appID, Path: "migration", Message: e.Error()}}, nil
	}
	id := uid.New()
	r.App.ReleaseID = id
	if s.OpenAPI != nil {
		r.App.OpenAPI, e = s.OpenAPI(r.App)
		if e != nil {
			return Published{}, nil, e
		}
	}
	appJSON, _ := json.Marshal(r.App)
	planJSON, _ := json.Marshal(plan)
	checksums := map[string]string{}
	defs, _ := s.Draft(ctx, appID)
	for _, d := range defs {
		k := d.Kind + "/" + d.Metadata.Name
		checksums[k], _ = definition.Checksum(d)
	}
	checkJSON, _ := json.Marshal(checksums)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = migration.Apply(ctx, s.Migrations, plan); e != nil {
		return Published{}, nil, fmt.Errorf("migration failed; previous release remains active: %w", e)
	}
	e = s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		if _, e := tx.Insert(ctx, dbal.Insert{Table: "bean_release", Values: map[string]dbal.Value{"id": id, "app_id": appID, "version": r.App.Version, "checksums": string(checkJSON), "app_ir": string(appJSON), "migration_plan": string(planJSON), "openapi": string(r.App.OpenAPI), "created_at": now, "activated_at": now, "status": "active"}}); e != nil {
			return e
		}
		rows, e := tx.Select(ctx, dbal.Select{Table: "bean_active_release", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, Limit: 1})
		if e != nil {
			return e
		}
		if len(rows) == 0 {
			_, e = tx.Insert(ctx, dbal.Insert{Table: "bean_active_release", Values: map[string]dbal.Value{"app_id": appID, "release_id": id}})
		} else {
			_, e = tx.Update(ctx, dbal.Update{Table: "bean_active_release", Values: map[string]dbal.Value{"release_id": id}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, ExpectedRows: 1})
		}
		return e
	})
	if e != nil {
		return Published{}, nil, e
	}
	if e = s.Kernel.Activate(r.App); e != nil {
		return Published{}, nil, e
	}
	return Published{ID: id, Version: r.App.Version, Status: "active", CreatedAt: now, ActivatedAt: now}, nil, nil
}
func (s *Store) LoadActive(ctx context.Context, appID string) error {
	a, e := s.activeApp(ctx, appID)
	if e != nil {
		return e
	}
	if a == nil {
		return nil
	}
	return s.Kernel.Activate(a)
}
func (s *Store) activeApp(ctx context.Context, appID string) (*appir.App, error) {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_active_release", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, Limit: 1})
	if e != nil || len(rows) == 0 {
		return nil, e
	}
	rr, e := s.DB.Select(ctx, dbal.Select{Table: "bean_release", Columns: []string{"app_ir"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: rows[0]["release_id"]}, Limit: 1})
	if e != nil || len(rr) == 0 {
		return nil, e
	}
	var a appir.App
	e = json.Unmarshal([]byte(fmt.Sprint(rr[0]["app_ir"])), &a)
	return &a, e
}
func (s *Store) Releases(ctx context.Context, appID string) ([]Published, error) {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_release", Columns: []string{"id", "version", "status", "created_at", "activated_at"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, OrderBy: []dbal.Order{{Column: "version", Desc: true}}, Limit: 200})
	if e != nil {
		return nil, e
	}
	out := []Published{}
	for _, r := range rows {
		out = append(out, Published{ID: fmt.Sprint(r["id"]), Version: int(asInt(r["version"])), Status: fmt.Sprint(r["status"]), CreatedAt: fmt.Sprint(r["created_at"]), ActivatedAt: fmt.Sprint(r["activated_at"])})
	}
	return out, nil
}
func (s *Store) nextVersion(ctx context.Context, appID string) int {
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_release", Columns: []string{"version"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, OrderBy: []dbal.Order{{Column: "version", Desc: true}}, Limit: 1})
	if e != nil || len(rows) == 0 {
		return 1
	}
	return int(asInt(rows[0]["version"])) + 1
}
func schemaOf(a *appir.App) migration.Schema {
	s := migration.Schema{}
	for _, e := range a.Entities {
		m := migration.Entity{Name: e.Name, Indexes: e.Indexes, Unique: e.Unique}
		for _, f := range e.Fields {
			mf := migration.Field{Name: f.Name, Type: f.Type, Required: f.Required, Unique: f.Unique}
			if f.Relation != nil {
				mf.RelationEntity, mf.RelationKind = f.Relation.Entity, f.Relation.Kind
			}
			m.Fields = append(m.Fields, mf)
		}
		if e.Owner {
			m.Fields = append(m.Fields, migration.Field{Name: "owner_id", Type: "uuid"})
		}
		if e.Tenant {
			m.Fields = append(m.Fields, migration.Field{Name: "tenant_id", Type: "uuid"})
		}
		if e.SoftDelete {
			m.Fields = append(m.Fields, migration.Field{Name: "deleted_at", Type: "datetime"})
		}
		s.Entities = append(s.Entities, m)
	}
	sort.Slice(s.Entities, func(i, j int) bool { return s.Entities[i].Name < s.Entities[j].Name })
	return s
}
func asInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}
