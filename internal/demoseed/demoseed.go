package demoseed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/view"
)

type Record struct {
	Entity string         `json:"entity"`
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

type Result struct {
	Name     string `json:"name"`
	Seed     int64  `json:"seed"`
	Records  int    `json:"records"`
	Checksum string `json:"checksum"`
}

func Generate(app *appir.App, seed int64) ([]Record, error) {
	if app.DemoSeed == nil {
		return nil, fmt.Errorf("application has no DemoSeed definition")
	}
	order, err := entityOrder(app)
	if err != nil {
		return nil, err
	}
	ids := map[string][]string{}
	records := []Record{}
	for _, entityName := range order {
		entity := app.Entities[entityName]
		definition := app.DemoSeed.Entities[entityName]
		for index := 0; index < definition.Count; index++ {
			id := stableUUID(seed, entityName, index)
			values := map[string]any{}
			for _, field := range entity.Fields {
				if field.Sensitive || field.Type == "password" || field.Type == "file" {
					continue
				}
				if field.Relation != nil {
					value := relationValue(field, index, ids)
					if value != nil {
						values[field.Name] = value
					}
					continue
				}
				if field.Required || includeOptional(index, field.Name) {
					values[field.Name] = scalarValue(field, definition.Profile, index, seed)
				}
			}
			records = append(records, Record{Entity: entityName, ID: id, Values: values})
			ids[entityName] = append(ids[entityName], id)
		}
	}
	return records, nil
}

func Run(ctx context.Context, database dbal.Database, app *appir.App, seed int64) (Result, error) {
	records, err := Generate(app, seed)
	if err != nil {
		return Result{}, err
	}
	result, err := resultFor(app.DemoSeed.Name, seed, records)
	if err != nil {
		return Result{}, err
	}
	roles := []string{"administrator"}
	for name := range app.Roles {
		if name != "administrator" {
			roles = append(roles, name)
		}
	}
	sort.Strings(roles)
	request := beanctx.Request{User: &beanctx.User{ID: stableUUID(seed, "owner", 0), Email: "demo@bean.local", Roles: roles}, TenantID: stableUUID(seed, "tenant", 0), RequestID: fmt.Sprintf("demo-seed:%d", seed)}
	empty, exact, err := inspectTarget(ctx, database, app, records, request)
	if err != nil {
		return Result{}, err
	}
	if exact {
		return result, nil
	}
	if !empty {
		return Result{}, fmt.Errorf("refusing to seed a non-empty target that does not match the generated dataset")
	}
	ids := map[string][]string{}
	for _, record := range records {
		ids[record.Entity] = append(ids[record.Entity], record.ID)
	}
	nextID := map[string]int{}
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(time.Duration(seed%8760) * time.Hour)
	service := action.Service{
		DB:  database,
		Now: func() time.Time { return clock },
		CreateID: func(entity appir.Entity, input map[string]any) string {
			index := nextID[entity.Name]
			nextID[entity.Name] = index + 1
			return ids[entity.Name][index]
		},
	}
	for _, record := range records {
		if _, err = service.Execute(ctx, app, record.Entity+"_create", record.Values, request); err != nil {
			return Result{}, fmt.Errorf("seed %s: %w", record.Entity, err)
		}
	}
	return result, nil
}

func inspectTarget(ctx context.Context, database dbal.Database, app *appir.App, records []Record, request beanctx.Request) (bool, bool, error) {
	expected := map[string]map[string]Record{}
	for _, record := range records {
		if expected[record.Entity] == nil {
			expected[record.Entity] = map[string]Record{}
		}
		expected[record.Entity][record.ID] = record
	}
	empty := true
	views := view.Service{DB: database}
	for entityName := range app.DemoSeed.Entities {
		page, err := views.RunPage(ctx, app, entityName+"_list", view.Params{Limit: 200}, request)
		if err != nil {
			return false, false, fmt.Errorf("verify seeded View %s_list: %w", entityName, err)
		}
		if len(page.Rows) > 0 {
			empty = false
		}
		if len(page.Rows) != len(expected[entityName]) {
			return empty, false, nil
		}
		entity := app.Entities[entityName]
		for _, row := range page.Rows {
			record, exists := expected[entityName][fmt.Sprint(row["id"])]
			if !exists {
				return empty, false, nil
			}
			for _, field := range entity.Fields {
				value, generated := record.Values[field.Name]
				if !generated && emptyValue(row[field.Name]) {
					continue
				}
				if canonical(row[field.Name]) != canonical(value) {
					return empty, false, nil
				}
			}
		}
	}
	return empty, !empty, nil
}

func emptyValue(value any) bool {
	if value == nil {
		return true
	}
	if values, ok := value.([]any); ok {
		return len(values) == 0
	}
	return false
}

func resultFor(name string, seed int64, records []Record) (Result, error) {
	encoded, err := json.Marshal(records)
	if err != nil {
		return Result{}, err
	}
	sum := sha256.Sum256(encoded)
	return Result{Name: name, Seed: seed, Records: len(records), Checksum: hex.EncodeToString(sum[:])}, nil
}

func entityOrder(app *appir.App) ([]string, error) {
	remaining := map[string]bool{}
	for entity := range app.DemoSeed.Entities {
		remaining[entity] = true
	}
	order := []string{}
	for len(remaining) > 0 {
		ready := []string{}
		fallback := []string{}
		for name := range remaining {
			blocked, requiredBlocked := false, false
			for _, field := range app.Entities[name].Fields {
				if field.Relation != nil && field.Relation.Entity != name && remaining[field.Relation.Entity] {
					blocked = true
					requiredBlocked = requiredBlocked || field.Required
				}
				if field.Required && field.Relation != nil && field.Relation.Entity == name {
					return nil, fmt.Errorf("required self relation %s.%s cannot be demo seeded", name, field.Name)
				}
			}
			if !blocked {
				ready = append(ready, name)
			}
			if !requiredBlocked {
				fallback = append(fallback, name)
			}
		}
		if len(ready) == 0 {
			sort.Strings(fallback)
			if len(fallback) == 0 {
				return nil, fmt.Errorf("demo seed required relations contain a cycle")
			}
			ready = fallback[:1]
		}
		sort.Strings(ready)
		for _, name := range ready {
			order = append(order, name)
			delete(remaining, name)
		}
	}
	return order, nil
}

func relationValue(field appir.Field, index int, ids map[string][]string) any {
	targets := ids[field.Relation.Entity]
	if len(targets) == 0 {
		return nil
	}
	if field.Relation.Kind == "one-to-many" || field.Relation.Kind == "many-to-many" {
		return []any{targets[index%len(targets)]}
	}
	return targets[index%len(targets)]
}

func scalarValue(field appir.Field, profile string, index int, seed int64) any {
	n := index + 1
	label := field.Label
	if label == "" {
		label = strings.ReplaceAll(field.Name, "_", " ")
	}
	switch field.Type {
	case "string", "text", "richtext":
		if field.Type == "text" || field.Type == "richtext" {
			return fmt.Sprintf("Demo %s %d for a credible %s workflow.", label, n, profile)
		}
		return namedValue(field.Name, profile, n)
	case "slug":
		return fmt.Sprintf("%s-%d", strings.ReplaceAll(field.Name, "_", "-"), n)
	case "integer", "money":
		return n * 10
	case "decimal":
		return fmt.Sprintf("%d.50", n)
	case "boolean":
		return index%2 == 0
	case "enum":
		if len(field.Options) > 0 {
			return field.Options[index%len(field.Options)]
		}
	case "date":
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, index).Format("2006-01-02")
	case "datetime":
		return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC).Add(time.Duration(index) * 24 * time.Hour).Format(time.RFC3339)
	case "email":
		return fmt.Sprintf("demo%d@example.test", n)
	case "url":
		return fmt.Sprintf("https://example.test/%s/%d", field.Name, n)
	case "uuid":
		return stableUUID(seed, field.Name, index)
	case "json":
		return map[string]any{"demo": true, "index": n}
	}
	return nil
}

func namedValue(name, profile string, n int) string {
	if profile == "people" || strings.Contains(name, "name") {
		first := []string{"Avery", "Jordan", "Morgan", "Riley", "Taylor", "Casey"}
		last := []string{"Nguyen", "Smith", "Patel", "Garcia", "Brown", "Wilson"}
		return fmt.Sprintf("%s %s %d", first[(n-1)%len(first)], last[(n-1)%len(last)], n)
	}
	prefix := map[string]string{"activities": "Follow-up", "companies": "Northstar", "jobs": "Product role", "notes": "Interview note"}[profile]
	if prefix == "" {
		prefix = strings.ReplaceAll(name, "_", " ")
	}
	return fmt.Sprintf("%s %d", prefix, n)
}

func includeOptional(index int, name string) bool {
	return (index+len(name))%3 != 0
}

func stableUUID(seed int64, namespace string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%d", seed, namespace, index)))
	sum[6] = (sum[6] & 0x0f) | 0x40
	sum[8] = (sum[8] & 0x3f) | 0x80
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func canonical(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
