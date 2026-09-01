package patterns

import (
	"encoding/json"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
)

func TestEveryPatternIsStableAndCompilesIndependently(t *testing.T) {
	if len(Names()) != 10 {
		t.Fatalf("patterns=%v", Names())
	}
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			first, err := Inspect(name)
			if err != nil {
				t.Fatal(err)
			}
			second, _ := Inspect(name)
			left, _ := json.Marshal(first)
			right, _ := json.Marshal(second)
			if string(left) != string(right) {
				t.Fatal("pattern output is not stable")
			}
			if diagnostics := compiler.Compile(name, 1, first.Definitions).Diagnostics; len(diagnostics) != 0 {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
		})
	}
}
