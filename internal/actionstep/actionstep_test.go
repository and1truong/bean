package actionstep

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
)

func TestSpecificationsDeclareStableEffects(t *testing.T) {
	want := []string{"assert", "assert_no_overlap", "conditional_update", "create", "decrement", "delete", "emit", "load", "query", "return", "schedule", "transition", "update"}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("steps=%v want=%v", got, want)
	}
	for _, name := range want {
		specification, exists := Lookup(name)
		if !exists {
			t.Fatalf("step %s is missing", name)
		}
		entityEffect := specification.Effects.ReadsEntity || specification.Effects.MutatesEntity
		if specification.UsesEntity != entityEffect {
			t.Fatalf("step %s entity declaration and effects drifted: %+v", name, specification)
		}
	}
	transition, _ := Lookup("transition")
	if !transition.Transition || !transition.Effects.MutatesEntity || transition.Effects.EmitsEvent || transition.Effects.SchedulesJob {
		t.Fatalf("transition effects=%+v", transition)
	}
	emit, _ := Lookup("emit")
	if !emit.Effects.EmitsEvent || emit.Effects.MutatesEntity {
		t.Fatalf("emit effects=%+v", emit.Effects)
	}
	schedule, _ := Lookup("schedule")
	if !schedule.Effects.SchedulesJob || schedule.Effects.MutatesEntity {
		t.Fatalf("schedule effects=%+v", schedule.Effects)
	}
}

func TestEntityNameUsesOneExplicitPrecedenceRule(t *testing.T) {
	action := appir.Action{Entity: "action_entity"}
	legacy := appir.Assignment{Field: "entity", Value: appir.ValueBinding{Source: "literal", Literal: json.RawMessage(`"legacy_entity"`)}}
	if got := EntityName(action, appir.Step{Values: []appir.Assignment{legacy}}); got != "legacy_entity" {
		t.Fatalf("legacy entity=%q", got)
	}
	if got := EntityName(action, appir.Step{Entity: "explicit_entity", Values: []appir.Assignment{legacy}}); got != "explicit_entity" {
		t.Fatalf("explicit entity=%q", got)
	}
}

func TestLookupDoesNotExposeMutableMetadata(t *testing.T) {
	first, _ := Lookup("update")
	first.AllowedValues[0] = "changed"
	second, _ := Lookup("update")
	if second.AllowedValues[0] == "changed" {
		t.Fatal("registered step metadata was mutable")
	}
}
