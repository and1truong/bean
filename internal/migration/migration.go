package migration

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type Field struct {
	Name, Type                   string
	RelationEntity, RelationKind string
	Required, Unique             bool
}
type Entity struct {
	Name    string
	Fields  []Field
	Indexes [][]string
	Unique  [][]string
}
type Schema struct{ Entities []Entity }
type Plan struct {
	Statements   []string
	Descriptions []string
}
type Inspector interface {
	Tables(context.Context) ([]string, error)
	Columns(context.Context, string) ([]Column, error)
}
type Column struct {
	Name, LogicalType string
	Nullable          bool
}
type Executor interface {
	ExecuteMigration(context.Context, []string) error
}

var ident = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func Build(current Schema, next Schema) (Plan, error) {
	old := map[string]Entity{}
	for _, e := range current.Entities {
		old[e.Name] = e
	}
	newNames := map[string]bool{}
	p := Plan{}
	for _, e := range next.Entities {
		if !ident.MatchString(e.Name) {
			return p, fmt.Errorf("Entity %s spec.name: invalid machine name", e.Name)
		}
		newNames[e.Name] = true
		prev, exists := old[e.Name]
		if !exists {
			sql, err := createTable(e)
			if err != nil {
				return p, err
			}
			p.Statements = append(p.Statements, sql)
			p.Descriptions = append(p.Descriptions, "create entity "+e.Name)
			addIndexes(&p, e)
			addJoinTables(&p, e)
			continue
		}
		pf := map[string]Field{}
		for _, f := range prev.Fields {
			pf[f.Name] = f
		}
		seen := map[string]bool{}
		for _, f := range e.Fields {
			seen[f.Name] = true
			o, ok := pf[f.Name]
			if !ok {
				if f.RelationKind == "many-to-many" {
					addJoinTable(&p, e.Name, f)
					continue
				}
				if f.Required {
					return p, fmt.Errorf("Entity %s spec.fields.%s: required fields need a default on existing tables", e.Name, f.Name)
				}
				typ, er := sqlType(f.Type)
				if er != nil {
					return p, er
				}
				p.Statements = append(p.Statements, `ALTER TABLE "`+e.Name+`" ADD COLUMN "`+f.Name+`" `+typ)
				p.Descriptions = append(p.Descriptions, "add field "+e.Name+"."+f.Name)
			} else if o.Type != f.Type {
				return p, fmt.Errorf("Entity %s spec.fields.%s: incompatible type change %s to %s", e.Name, f.Name, o.Type, f.Type)
			}
		}
		for name := range pf {
			if !seen[name] {
				return p, fmt.Errorf("Entity %s spec.fields.%s: destructive field deletion is blocked", e.Name, name)
			}
		}
		addNewIndexes(&p, prev, e)
	}
	for name := range old {
		if !newNames[name] {
			return p, fmt.Errorf("Entity %s: entity deletion is blocked", name)
		}
	}
	return p, nil
}
func createTable(e Entity) (string, error) {
	cols := []string{`"id" TEXT PRIMARY KEY`, `"created_at" TEXT NOT NULL`, `"updated_at" TEXT NOT NULL`, `"version" INTEGER NOT NULL`}
	for _, f := range e.Fields {
		if f.RelationKind == "many-to-many" {
			continue
		}
		if !ident.MatchString(f.Name) {
			return "", fmt.Errorf("Entity %s spec.fields.%s: invalid field name", e.Name, f.Name)
		}
		t, er := sqlType(f.Type)
		if er != nil {
			return "", fmt.Errorf("Entity %s spec.fields.%s: %w", e.Name, f.Name, er)
		}
		c := `"` + f.Name + `" ` + t
		if f.Required {
			c += " NOT NULL"
		}
		if f.Unique {
			c += " UNIQUE"
		}
		cols = append(cols, c)
	}
	for _, fields := range e.Unique {
		quoted := []string{}
		for _, name := range fields {
			if !ident.MatchString(name) {
				return "", fmt.Errorf("Entity %s spec.unique: invalid field %s", e.Name, name)
			}
			quoted = append(quoted, `"`+name+`"`)
		}
		if len(quoted) > 0 {
			cols = append(cols, "UNIQUE ("+strings.Join(quoted, ",")+")")
		}
	}
	return `CREATE TABLE "` + e.Name + `" (` + strings.Join(cols, ",") + `)`, nil
}
func addIndexes(p *Plan, e Entity) {
	for i, fields := range e.Indexes {
		addIndex(p, e.Name, fields, false, i)
	}
}
func addNewIndexes(p *Plan, old, next Entity) {
	seen := map[string]bool{}
	for _, fields := range old.Indexes {
		seen[strings.Join(fields, ",")] = true
	}
	for i, fields := range next.Indexes {
		if !seen[strings.Join(fields, ",")] {
			addIndex(p, next.Name, fields, false, i)
		}
	}
	seen = map[string]bool{}
	for _, fields := range old.Unique {
		seen[strings.Join(fields, ",")] = true
	}
	for i, fields := range next.Unique {
		if !seen[strings.Join(fields, ",")] {
			addIndex(p, next.Name, fields, true, i)
		}
	}
}
func addIndex(p *Plan, entity string, fields []string, unique bool, n int) {
	if len(fields) == 0 {
		return
	}
	quoted := []string{}
	for _, field := range fields {
		if !ident.MatchString(field) {
			return
		}
		quoted = append(quoted, `"`+field+`"`)
	}
	prefix, kind := "", "index"
	if unique {
		prefix, kind = "UNIQUE ", "unique constraint"
	}
	name := fmt.Sprintf("bean_%s_%d_%s", entity, n, strings.Join(fields, "_"))
	p.Statements = append(p.Statements, "CREATE "+prefix+`INDEX "`+name+`" ON "`+entity+`" (`+strings.Join(quoted, ",")+")")
	p.Descriptions = append(p.Descriptions, "add "+kind+" "+name)
}
func addJoinTables(p *Plan, e Entity) {
	for _, field := range e.Fields {
		if field.RelationKind == "many-to-many" {
			addJoinTable(p, e.Name, field)
		}
	}
}
func addJoinTable(p *Plan, entity string, field Field) {
	if !ident.MatchString(field.RelationEntity) {
		return
	}
	name := entity + "_" + field.Name
	statement := `CREATE TABLE "` + name + `" ("` + entity + `_id" TEXT NOT NULL,"` + field.RelationEntity + `_id" TEXT NOT NULL,PRIMARY KEY("` + entity + `_id","` + field.RelationEntity + `_id"),FOREIGN KEY("` + entity + `_id") REFERENCES "` + entity + `"("id"),FOREIGN KEY("` + field.RelationEntity + `_id") REFERENCES "` + field.RelationEntity + `"("id"))`
	p.Statements = append(p.Statements, statement)
	p.Descriptions = append(p.Descriptions, "create many-to-many relation "+name)
}
func sqlType(t string) (string, error) {
	switch t {
	case "string", "text", "richtext", "enum", "date", "datetime", "uuid", "email", "url", "json", "relation":
		return "TEXT", nil
	case "integer", "money", "boolean":
		return "INTEGER", nil
	case "decimal":
		return "TEXT", nil
	default:
		return "", fmt.Errorf("unsupported field type %q", t)
	}
}
func Apply(ctx context.Context, e Executor, p Plan) error {
	return e.ExecuteMigration(ctx, p.Statements)
}
func MetadataSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS bean_app (id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bean_definition (id TEXT PRIMARY KEY, app_id TEXT NOT NULL, kind TEXT NOT NULL, namespace TEXT NOT NULL, name TEXT NOT NULL, current_revision INTEGER NOT NULL, UNIQUE(app_id,kind,namespace,name))`,
		`CREATE TABLE IF NOT EXISTS bean_definition_revision (definition_id TEXT NOT NULL, revision INTEGER NOT NULL, checksum TEXT NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(definition_id,revision))`,
		`CREATE TABLE IF NOT EXISTS bean_release (id TEXT PRIMARY KEY, app_id TEXT NOT NULL, version INTEGER NOT NULL, checksums TEXT NOT NULL, app_ir TEXT NOT NULL, migration_plan TEXT NOT NULL, openapi TEXT NOT NULL, created_at TEXT NOT NULL, activated_at TEXT, status TEXT NOT NULL, UNIQUE(app_id,version))`,
		`CREATE TABLE IF NOT EXISTS bean_release_definition (release_id TEXT NOT NULL, definition_id TEXT NOT NULL, revision INTEGER NOT NULL, PRIMARY KEY(release_id,definition_id))`,
		`CREATE TABLE IF NOT EXISTS bean_schema_migration (release_id TEXT NOT NULL, sequence INTEGER NOT NULL, description TEXT NOT NULL, applied_at TEXT NOT NULL, PRIMARY KEY(release_id,sequence))`,
		`CREATE TABLE IF NOT EXISTS bean_active_release (app_id TEXT PRIMARY KEY, release_id TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bean_user (id TEXT PRIMARY KEY,email TEXT NOT NULL UNIQUE,password_hash TEXT NOT NULL,roles TEXT NOT NULL,tenant_id TEXT,created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bean_session (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,csrf_token TEXT NOT NULL,expires_at TEXT NOT NULL,FOREIGN KEY(user_id) REFERENCES bean_user(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS bean_audit (id TEXT PRIMARY KEY,at TEXT NOT NULL,request_id TEXT,user_id TEXT,tenant_id TEXT,action TEXT NOT NULL,entity_type TEXT,entity_id TEXT,changed_fields TEXT,success INTEGER NOT NULL,error TEXT)`,
		`CREATE TABLE IF NOT EXISTS bean_outbox (id TEXT PRIMARY KEY,topic TEXT NOT NULL,payload TEXT NOT NULL,created_at TEXT NOT NULL,delivered_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS bean_job (id TEXT PRIMARY KEY,name TEXT NOT NULL,run_at TEXT NOT NULL,status TEXT NOT NULL,payload TEXT NOT NULL,attempts INTEGER NOT NULL,retry_delay INTEGER NOT NULL,last_error TEXT,completed_at TEXT)`,
	}
}
