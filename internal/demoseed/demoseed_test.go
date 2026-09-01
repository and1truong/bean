package demoseed

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	fieldpkg "github.com/beanruntime/bean/internal/field"
)

func TestGenerateIsDeterministicAndOrdersRelations(t *testing.T) {
	app := appir.Empty()
	app.Entities["company"] = appir.Entity{Name: "company", Fields: []appir.Field{{Name: "name", Label: "Name", Type: "string", Required: true}}}
	app.Entities["candidate"] = appir.Entity{Name: "candidate", Fields: []appir.Field{
		{Name: "name", Label: "Name", Type: "string", Required: true},
		{Name: "company_id", Type: "relation", Required: true, Relation: &appir.Relation{Entity: "company", Kind: "many-to-one", TargetField: "id"}},
		{Name: "stage", Type: "enum", Required: true, Options: []string{"applied", "interview"}},
	}}
	app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"candidate": {Count: 3, Profile: "people"}, "company": {Count: 2, Profile: "companies"}}}

	first, err := Generate(app, 42)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(app, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different records")
	}
	if len(first) != 5 || first[0].Entity != "company" || first[2].Entity != "candidate" {
		t.Fatalf("records=%+v", first)
	}
	if first[2].Values["company_id"] != first[0].ID {
		t.Fatalf("relation=%v want %s", first[2].Values["company_id"], first[0].ID)
	}
}

func TestGenerateCoversSupportedScalarTypesAndSkipsUnsafeFields(t *testing.T) {
	app := appir.Empty()
	fields := []appir.Field{
		{Name: "name", Type: "string", Required: true, Unique: true},
		{Name: "text", Type: "text", Required: true}, {Name: "rich", Type: "richtext", Required: true},
		{Name: "slug", Type: "slug", Required: true}, {Name: "integer", Type: "integer", Required: true},
		{Name: "money", Type: "money", Required: true}, {Name: "decimal", Type: "decimal", Required: true},
		{Name: "boolean", Type: "boolean", Required: true}, {Name: "enum", Type: "enum", Required: true, Options: []string{"one", "two"}},
		{Name: "date", Type: "date", Required: true}, {Name: "datetime", Type: "datetime", Required: true},
		{Name: "email", Type: "email", Required: true, Unique: true}, {Name: "url", Type: "url", Required: true},
		{Name: "uuid", Type: "uuid", Required: true}, {Name: "json", Type: "json", Required: true},
		{Name: "password", Type: "password"}, {Name: "file", Type: "file"}, {Name: "secret", Type: "string", Sensitive: true},
	}
	app.Entities["sample"] = appir.Entity{Name: "sample", Fields: fields}
	app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"sample": {Count: 3, Profile: "auto"}}}
	records, err := Generate(app, 7)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		for _, definition := range fields[:15] {
			if err := fieldpkg.Validate(definition, record.Values[definition.Name]); err != nil {
				t.Fatalf("%s=%v: %v", definition.Name, record.Values[definition.Name], err)
			}
		}
		for _, name := range []string{"password", "file", "secret"} {
			if _, exists := record.Values[name]; exists {
				t.Fatalf("unsafe field %s was generated", name)
			}
		}
	}
	if records[0].Values["name"] == records[1].Values["name"] || records[0].Values["email"] == records[1].Values["email"] {
		t.Fatal("unique values were repeated")
	}
}
