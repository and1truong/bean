package migration

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/dbal"
)

type Field struct {
	Name, Type                                string
	RelationEntity, RelationKind, TargetField string
	Required, Unique                          bool
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
	Dialect() string
	Tables(context.Context) ([]string, error)
	Columns(context.Context, string) ([]dbal.Column, error)
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
	}
	created := orderedNewEntities(old, next.Entities)
	for _, e := range created {
		sql, err := createTable(e)
		if err != nil {
			return p, err
		}
		p.Statements = append(p.Statements, sql)
		p.Descriptions = append(p.Descriptions, "create entity "+e.Name)
		addIndexes(&p, e)
	}
	for _, e := range created {
		addJoinTables(&p, e)
	}
	for _, e := range next.Entities {
		prev, exists := old[e.Name]
		if !exists {
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
				if toMany(f.RelationKind) {
					addJoinTable(&p, e.Name, f)
					continue
				}
				if f.Required {
					return p, fmt.Errorf("Entity %s spec.fields.%s: required fields need a default on existing tables", e.Name, f.Name)
				}
				if f.Unique || f.RelationEntity != "" {
					return p, fmt.Errorf("Entity %s spec.fields.%s: constrained fields cannot be added to an existing table safely", e.Name, f.Name)
				}
				typ, er := sqlType(f.Type)
				if er != nil {
					return p, er
				}
				p.Statements = append(p.Statements, `ALTER TABLE "`+e.Name+`" ADD COLUMN "`+f.Name+`" `+typ)
				p.Descriptions = append(p.Descriptions, "add field "+e.Name+"."+f.Name)
			} else if o.Type != f.Type || o.Required != f.Required || o.Unique != f.Unique || o.RelationEntity != f.RelationEntity || o.RelationKind != f.RelationKind || o.TargetField != f.TargetField {
				return p, fmt.Errorf("Entity %s spec.fields.%s: incompatible field contract change", e.Name, f.Name)
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

func orderedNewEntities(old map[string]Entity, entities []Entity) []Entity {
	pending := map[string]Entity{}
	for _, entity := range entities {
		if _, exists := old[entity.Name]; !exists {
			pending[entity.Name] = entity
		}
	}
	ordered := []Entity{}
	for len(pending) > 0 {
		names := make([]string, 0, len(pending))
		for name := range pending {
			names = append(names, name)
		}
		sort.Strings(names)
		progress := false
		for _, name := range names {
			entity := pending[name]
			blocked := false
			for _, field := range entity.Fields {
				if !toMany(field.RelationKind) && field.RelationEntity != "" && field.RelationEntity != entity.Name {
					_, blocked = pending[field.RelationEntity]
				}
				if blocked {
					break
				}
			}
			if blocked {
				continue
			}
			ordered = append(ordered, entity)
			delete(pending, name)
			progress = true
		}
		if !progress {
			for _, name := range names {
				ordered = append(ordered, pending[name])
				delete(pending, name)
			}
		}
	}
	return ordered
}
func createTable(e Entity) (string, error) {
	cols := []string{`"id" TEXT PRIMARY KEY`, `"created_at" TEXT NOT NULL`, `"updated_at" TEXT NOT NULL`, `"version" INTEGER NOT NULL`}
	constraints := []string{}
	for _, f := range e.Fields {
		if toMany(f.RelationKind) {
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
		if f.Unique || f.RelationKind == "one-to-one" {
			c += " UNIQUE"
		}
		cols = append(cols, c)
		if f.RelationEntity != "" {
			target := f.TargetField
			if target == "" {
				target = "id"
			}
			constraints = append(constraints, `FOREIGN KEY("`+f.Name+`") REFERENCES "`+f.RelationEntity+`"("`+target+`")`)
		}
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
	cols = append(cols, constraints...)
	return `CREATE TABLE IF NOT EXISTS "` + e.Name + `" (` + strings.Join(cols, ",") + `)`, nil
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
	p.Statements = append(p.Statements, "CREATE "+prefix+`INDEX IF NOT EXISTS "`+name+`" ON "`+entity+`" (`+strings.Join(quoted, ",")+")")
	p.Descriptions = append(p.Descriptions, "add "+kind+" "+name)
}
func addJoinTables(p *Plan, e Entity) {
	for _, field := range e.Fields {
		if toMany(field.RelationKind) {
			addJoinTable(p, e.Name, field)
		}
	}
}
func addJoinTable(p *Plan, entity string, field Field) {
	if !ident.MatchString(field.RelationEntity) {
		return
	}
	name := entity + "_" + field.Name
	target := field.TargetField
	if target == "" {
		target = "id"
	}
	uniqueTarget := ""
	if field.RelationKind == "one-to-many" {
		uniqueTarget = `,UNIQUE("` + field.RelationEntity + `_id")`
	}
	statement := `CREATE TABLE IF NOT EXISTS "` + name + `" ("` + entity + `_id" TEXT NOT NULL,"` + field.RelationEntity + `_id" TEXT NOT NULL,PRIMARY KEY("` + entity + `_id","` + field.RelationEntity + `_id")` + uniqueTarget + `,FOREIGN KEY("` + entity + `_id") REFERENCES "` + entity + `"("id"),FOREIGN KEY("` + field.RelationEntity + `_id") REFERENCES "` + field.RelationEntity + `"("` + target + `"))`
	p.Statements = append(p.Statements, statement)
	p.Descriptions = append(p.Descriptions, "create many-to-many relation "+name)
}
func toMany(kind string) bool { return kind == "one-to-many" || kind == "many-to-many" }
func sqlType(t string) (string, error) {
	switch t {
	case "string", "text", "richtext", "slug", "enum", "date", "datetime", "uuid", "email", "url", "json", "relation":
		return "TEXT", nil
	case "integer", "money":
		return "INTEGER", nil
	case "boolean":
		return "BOOLEAN", nil
	case "decimal":
		return "TEXT", nil
	default:
		return "", fmt.Errorf("unsupported field type %q", t)
	}
}
func Apply(ctx context.Context, e Executor, p Plan) error {
	return e.ExecuteMigration(ctx, p.Statements)
}

// Reconcile removes additive steps already visible in physical storage after an
// interrupted publish. Other statements are intrinsically idempotent.
func Reconcile(ctx context.Context, inspector Inspector, plan Plan) (Plan, error) {
	out := Plan{}
	for index, statement := range plan.Statements {
		description := plan.Descriptions[index]
		if strings.HasPrefix(description, "add field ") {
			parts := strings.Split(strings.TrimPrefix(description, "add field "), ".")
			if len(parts) != 2 {
				return Plan{}, fmt.Errorf("invalid migration description %q", description)
			}
			columns, err := inspector.Columns(ctx, parts[0])
			if err != nil {
				return Plan{}, err
			}
			alreadyApplied := false
			for _, column := range columns {
				if column.Name == parts[1] {
					alreadyApplied = true
					break
				}
			}
			if alreadyApplied {
				continue
			}
		}
		out.Statements = append(out.Statements, statement)
		out.Descriptions = append(out.Descriptions, description)
	}
	return out, nil
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
		`CREATE TABLE IF NOT EXISTS bean_user (id TEXT PRIMARY KEY,email TEXT NOT NULL UNIQUE,display_name TEXT,password_hash TEXT NOT NULL,roles TEXT NOT NULL,tenant_id TEXT,created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bean_session (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,csrf_token TEXT NOT NULL,expires_at TEXT NOT NULL,FOREIGN KEY(user_id) REFERENCES bean_user(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS bean_audit (id TEXT PRIMARY KEY,at TEXT NOT NULL,request_id TEXT,user_id TEXT,tenant_id TEXT,action TEXT NOT NULL,entity_type TEXT,entity_id TEXT,changed_fields TEXT,success INTEGER NOT NULL,error TEXT)`,
		`CREATE TABLE IF NOT EXISTS bean_outbox (id TEXT PRIMARY KEY,topic TEXT NOT NULL,payload TEXT NOT NULL,created_at TEXT NOT NULL,delivered_at TEXT,status TEXT NOT NULL,attempts INTEGER NOT NULL,retry_delay INTEGER NOT NULL,max_attempts INTEGER NOT NULL,last_error TEXT,claim_token TEXT,claimed_at TEXT,next_attempt_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS bean_job (id TEXT PRIMARY KEY,name TEXT NOT NULL,run_at TEXT NOT NULL,status TEXT NOT NULL,payload TEXT NOT NULL,attempts INTEGER NOT NULL,retry_delay INTEGER NOT NULL,last_error TEXT,completed_at TEXT,claim_token TEXT,claimed_at TEXT,next_attempt_at TEXT,max_attempts INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bean_idempotency (action TEXT NOT NULL,key TEXT NOT NULL,input_hash TEXT NOT NULL,result TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(action,key))`,
	}
}

// UpgradeMetadata adds columns introduced after the first metadata schema.
// Each statement is committed independently so startup can safely resume it.
func UpgradeMetadata(ctx context.Context, inspector Inspector, executor Executor) error {
	upgrades := map[string][]struct {
		name string
		sql  string
	}{
		"bean_user":        {{"display_name", `ALTER TABLE "bean_user" ADD COLUMN "display_name" TEXT`}},
		"bean_idempotency": {{"input_hash", `ALTER TABLE "bean_idempotency" ADD COLUMN "input_hash" TEXT`}},
		"bean_job": {
			{"claim_token", `ALTER TABLE "bean_job" ADD COLUMN "claim_token" TEXT`},
			{"claimed_at", `ALTER TABLE "bean_job" ADD COLUMN "claimed_at" TEXT`},
			{"next_attempt_at", `ALTER TABLE "bean_job" ADD COLUMN "next_attempt_at" TEXT`},
			{"max_attempts", `ALTER TABLE "bean_job" ADD COLUMN "max_attempts" INTEGER NOT NULL DEFAULT 5`},
		},
		"bean_outbox": {
			{"status", `ALTER TABLE "bean_outbox" ADD COLUMN "status" TEXT NOT NULL DEFAULT 'pending'`},
			{"attempts", `ALTER TABLE "bean_outbox" ADD COLUMN "attempts" INTEGER NOT NULL DEFAULT 0`},
			{"retry_delay", `ALTER TABLE "bean_outbox" ADD COLUMN "retry_delay" INTEGER NOT NULL DEFAULT 60`},
			{"max_attempts", `ALTER TABLE "bean_outbox" ADD COLUMN "max_attempts" INTEGER NOT NULL DEFAULT 10`},
			{"last_error", `ALTER TABLE "bean_outbox" ADD COLUMN "last_error" TEXT`},
			{"claim_token", `ALTER TABLE "bean_outbox" ADD COLUMN "claim_token" TEXT`},
			{"claimed_at", `ALTER TABLE "bean_outbox" ADD COLUMN "claimed_at" TEXT`},
			{"next_attempt_at", `ALTER TABLE "bean_outbox" ADD COLUMN "next_attempt_at" TEXT`},
		},
	}
	for table, candidates := range upgrades {
		columns, err := inspector.Columns(ctx, table)
		if err != nil {
			return err
		}
		exists := map[string]bool{}
		for _, column := range columns {
			exists[column.Name] = true
		}
		for _, candidate := range candidates {
			if exists[candidate.name] {
				continue
			}
			if err = executor.ExecuteMigration(ctx, []string{candidate.sql}); err != nil {
				return fmt.Errorf("upgrade metadata %s.%s: %w", table, candidate.name, err)
			}
		}
	}
	return nil
}
