package release_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestHistoricalReleaseFieldLayoutUpgrade(t *testing.T) {
	for _, version := range []int{14, 15, 16, 17} {
		t.Run(fmt.Sprintf("v%d-to-v18", version), func(t *testing.T) {
			ctx := context.Background()
			encoded, err := os.ReadFile(filepath.Join("..", "appir", "testdata", fmt.Sprintf("field-layout-baseline-v%d.json", version)))
			if err != nil {
				t.Fatal(err)
			}
			historical, err := appir.Decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			source, err := os.ReadFile(filepath.Join("..", "appir", "testdata", fmt.Sprintf("field-layout-baseline-v%d.source.json", version)))
			if err != nil {
				t.Fatal(err)
			}
			var definitions []definition.Definition
			if err = json.Unmarshal(source, &definitions); err != nil {
				t.Fatal(err)
			}
			bundle := definition.Bundle{Name: "Compatibility", Definitions: definitions}
			// Recompiling original definitions may only change the format and fill the
			// v15-owned default direction. No prior Action/View/Policy/auth behavior moves.
			compiled := compiler.Compile(historical.AppID, historical.Version, definitions)
			if len(compiled.Diagnostics) > 0 {
				t.Fatal(compiled.Diagnostics)
			}
			expected, err := historical.Clone()
			if err != nil {
				t.Fatal(err)
			}
			expected.FormatVersion = appir.CurrentFormat
			if version == 14 {
				sequence := expected.Sequences["intro"]
				for i := range sequence.Frames {
					sequence.Frames[i].Direction = "next"
				}
				expected.Sequences["intro"] = sequence
			}
			// Decode preserves dynamic numeric literals as json.Number, while the
			// compiler may produce Go integers. Compare canonical serialized values.
			expectedJSON, err := json.Marshal(expected)
			if err != nil {
				t.Fatal(err)
			}
			compiledJSON, err := json.Marshal(compiled.App)
			if err != nil {
				t.Fatal(err)
			}
			if string(expectedJSON) != string(compiledJSON) {
				t.Fatalf("recompilation changed more than format/default directions:\nexpected=%s\nactual=%s", expectedJSON, compiledJSON)
			}

			path := filepath.Join(t.TempDir(), "upgrade.db")
			database, err := sqlite.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			newStore := func(k *kernel.Kernel) *release.Store {
				return &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: k, OpenAPI: openapi.Generate, HostValidation: func(*appir.App) error { return nil }}
			}
			store := newStore(kernel.New())
			if err = store.Initialize(ctx); err != nil {
				t.Fatal(err)
			}
			initial, _, diagnostics, err := store.PublishBundle(ctx, historical.AppID, bundle)
			if err != nil || len(diagnostics) > 0 {
				t.Fatalf("initial publish: %v %v", err, diagnostics)
			}
			// Install an actual historical-compiler snapshot into isolated release
			// storage. Only release identity is adjusted, not its format or feature data.
			historical.ReleaseID = initial.ID
			body, err := json.Marshal(historical)
			if err != nil {
				t.Fatal(err)
			}
			err = database.Transaction(ctx, func(tx dbal.Transaction) error {
				_, err := tx.Update(ctx, dbal.Update{Table: "bean_release", Values: map[string]dbal.Value{"app_ir": string(body)}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: initial.ID}, ExpectedRows: 1})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}
			database, err = sqlite.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			runtime := kernel.New()
			store = newStore(runtime)
			if err = store.LoadActive(ctx, historical.AppID); err != nil {
				t.Fatal(err)
			}
			before, ok := runtime.Active()
			if !ok || !reflect.DeepEqual(before, historical) {
				t.Fatal("historical release not restored exactly")
			}

			layout := map[string]any{"groups": []any{map[string]any{"name": "content", "label": "Content", "columns": 2, "fields": []any{map[string]any{"field": "title", "span": "full"}}}}}
			definitions[1].Spec["displays"].(map[string]any)["detail"].(map[string]any)["renderer"] = map[string]any{"type": "detail", "layout": layout}
			definitions = append(definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "AdminResource", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"entity": "note", "form": map[string]any{"fields": []any{"title"}, "layout": layout}}})
			bundle.Definitions = definitions
			published, plan, diagnostics, err := store.PublishBundle(ctx, historical.AppID, bundle)
			if err != nil || len(diagnostics) > 0 || len(plan.Statements) > 0 || len(plan.Descriptions) > 0 {
				t.Fatalf("upgrade: %v %v plan=%+v", err, diagnostics, plan)
			}
			after, ok := runtime.Active()
			if !ok || after.ReleaseID != published.ID || after.FormatVersion != appir.CurrentFormat {
				t.Fatal("v18 not atomically activated")
			}
			if before.FormatVersion != historical.FormatVersion || before.AdminResources["note"].Form.Layout != nil {
				t.Fatal("upgrade mutated old snapshot")
			}
			if !reflect.DeepEqual(after.Authentication, before.Authentication) {
				t.Fatal("upgrade changed authentication")
			}
			if version >= 15 && !reflect.DeepEqual(after.Sequences, before.Sequences) {
				t.Fatal("upgrade changed directional navigation")
			}
			finalKernel := kernel.New()
			if err = newStore(finalKernel).LoadActive(ctx, historical.AppID); err != nil {
				t.Fatal(err)
			}
			restored, _ := finalKernel.Active()
			if restored.AdminResources["note"].Form.Layout == nil || restored.Views["notes"].Displays["detail"].Renderer.Layout == nil {
				t.Fatal("upgrade layout lost on restart")
			}

			// Invalid layout publication cannot replace the new active release.
			layout["groups"].([]any)[0].(map[string]any)["columns"] = 3
			_, _, diagnostics, err = store.PublishBundle(ctx, historical.AppID, bundle)
			if err != nil || len(diagnostics) == 0 {
				t.Fatalf("invalid publication: %v %v", err, diagnostics)
			}
			active, _ := runtime.Active()
			if active.ReleaseID != published.ID {
				t.Fatal("invalid layout replaced active release")
			}
		})
	}
}
