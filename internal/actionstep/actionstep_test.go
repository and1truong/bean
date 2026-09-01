package actionstep

import (
	"reflect"
	"testing"
)

func TestSpecificationsDeclareStableEffects(t *testing.T) {
	want := []string{"assert", "assert_no_overlap", "conditional_update", "create", "decrement", "delete", "emit", "load", "query", "return", "schedule", "transition", "update"}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("steps=%v want=%v", got, want)
	}
	for _, name := range want {
		if _, exists := Lookup(name); !exists {
			t.Fatalf("step %s is missing", name)
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

func TestLookupDoesNotExposeMutableMetadata(t *testing.T) {
	first, _ := Lookup("update")
	first.AllowedValues[0] = "changed"
	second, _ := Lookup("update")
	if second.AllowedValues[0] == "changed" {
		t.Fatal("registered step metadata was mutable")
	}
}
