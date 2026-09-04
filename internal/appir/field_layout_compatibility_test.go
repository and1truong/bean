package appir_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
)

func TestFieldLayoutFormatCompatibilityMatrix(t *testing.T) {
	// Exercise each gate independently and in combination, including unsupported
	// future formats. Empty snapshots of every historically supported format work.
	for version := 1; version <= 19; version++ {
		for mask := 0; mask < 32; mask++ {
			t.Run(fmt.Sprintf("v%d/features-%05b", version, mask), func(t *testing.T) {
				app := appir.Empty()
				app.FormatVersion = fmt.Sprintf("bean/appir/v%d", version)
				if mask&1 != 0 {
					app.Sequences["intro"] = appir.Sequence{Frames: []appir.SequenceFrame{{Direction: "next"}, {Direction: "down"}}}
				}
				if mask&2 != 0 {
					app.Authentication = &appir.Authentication{Preset: "internal"}
				}
				if mask&4 != 0 {
					app.AdminResources["note"] = appir.AdminResource{Form: appir.AdminForm{Layout: &appir.FieldLayout{}}}
				}
				if mask&8 != 0 {
					app.Views["notes"] = appir.View{Displays: map[string]appir.Display{"detail": {Renderer: appir.ViewRenderer{Layout: &appir.FieldLayout{}}}}}
				}
				if mask&16 != 0 {
					if app.Authentication == nil {
						app.Authentication = &appir.Authentication{Preset: "internal"}
					}
					app.Authentication.PasswordRecovery = true
				}
				before, err := json.Marshal(app)
				if err != nil {
					t.Fatal(err)
				}
				err = app.ValidateFormat()
				allowed := version <= 18 && (mask&1 == 0 || version >= 15) && (mask&2 == 0 || version >= 16) && (mask&12 == 0 || version >= 18) && (mask&16 == 0 || version >= 17)
				if (err == nil) != allowed {
					t.Fatalf("allowed=%v err=%v", allowed, err)
				}
				after, _ := json.Marshal(app)
				if string(before) != string(after) {
					t.Fatal("format validation mutated the snapshot")
				}
			})
		}
	}
}

func TestHistoricalCompilerSnapshotsRemainReadable(t *testing.T) {
	for _, version := range []int{14, 15, 16, 17} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			encoded, err := os.ReadFile(filepath.Join("testdata", fmt.Sprintf("field-layout-baseline-v%d.json", version)))
			if err != nil {
				t.Fatal(err)
			}
			app, err := appir.Decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if app.FormatVersion != fmt.Sprintf("bean/appir/v%d", version) {
				t.Fatal(app.FormatVersion)
			}
			if err = app.ValidateFormat(); err != nil {
				t.Fatal(err)
			}
			clone, err := app.Clone()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(app, clone) {
				t.Fatal("historical snapshot changed on clone")
			}
			if app.AdminResources["note"].Form.Layout != nil || app.Views["notes"].Displays["detail"].Renderer.Layout != nil {
				t.Fatal("legacy layout must stay absent")
			}
			frames := app.Sequences["intro"].Frames
			if len(frames) != 2 {
				t.Fatalf("missing historical frames: %+v", frames)
			}
			if version == 14 {
				if frames[1].Direction != "" {
					t.Fatal("v14 frame acquired a direction")
				}
			} else if frames[1].Direction != "down" {
				t.Fatal("direction lost")
			}
			if version >= 16 {
				if app.Authentication == nil || app.Authentication.Preset != "internal" || app.Authentication.Registration {
					t.Fatal("authentication changed")
				}
			} else if app.Authentication != nil {
				t.Fatal("authentication invented")
			}
			if app.PasswordRecoveryEnabled() != (version == 17) {
				t.Fatal("password recovery capability changed")
			}
			// Private v15/v17 field-layout prototypes are not the supported
			// directional Sequence and Password Recovery formats.
			resource := app.AdminResources["note"]
			resource.Form.Layout = &appir.FieldLayout{}
			app.AdminResources["note"] = resource
			if app.ValidateFormat() == nil {
				t.Fatal("old format accepted prototype field layout")
			}
		})
	}
}
