package actionstep

import "github.com/beanruntime/bean/internal/registry"

type Effects struct {
	ReadsEntity   bool
	MutatesEntity bool
	EmitsEvent    bool
	SchedulesJob  bool
}

type Specification struct {
	Effects               Effects
	UsesEntity            bool
	EntityFields          bool
	AllowedValues         []string
	AnyValues             bool
	OutputValues          bool
	RequiresID            bool
	RequiresCondition     bool
	RequiresView          bool
	RequiresJob           bool
	RequiresEvent         bool
	Transition            bool
	ProtectLifecycleState bool
}

var specifications = registry.Must(
	cloneSpecification,
	entry("assert", Specification{AllowedValues: []string{"message"}, RequiresCondition: true}),
	entry("assert_no_overlap", Specification{Effects: Effects{ReadsEntity: true}, UsesEntity: true, AllowedValues: []string{"match", "start", "end", "message"}}),
	entry("conditional_update", Specification{Effects: Effects{ReadsEntity: true, MutatesEntity: true}, UsesEntity: true, EntityFields: true, AllowedValues: []string{"entity", "id", "message"}, RequiresID: true, RequiresCondition: true, ProtectLifecycleState: true}),
	entry("create", Specification{Effects: Effects{MutatesEntity: true}, UsesEntity: true, EntityFields: true, AllowedValues: []string{"entity"}}),
	entry("decrement", Specification{Effects: Effects{ReadsEntity: true, MutatesEntity: true}, UsesEntity: true, AllowedValues: []string{"entity", "field", "id_input", "amount_input", "message"}}),
	entry("delete", Specification{Effects: Effects{ReadsEntity: true, MutatesEntity: true}, UsesEntity: true, AllowedValues: []string{"entity", "id"}, RequiresID: true}),
	entry("emit", Specification{Effects: Effects{EmitsEvent: true}, AnyValues: true, RequiresEvent: true}),
	entry("load", Specification{Effects: Effects{ReadsEntity: true}, UsesEntity: true, AllowedValues: []string{"entity", "id"}, RequiresID: true}),
	entry("query", Specification{Effects: Effects{ReadsEntity: true}, UsesEntity: true, RequiresView: true}),
	entry("return", Specification{OutputValues: true}),
	entry("schedule", Specification{Effects: Effects{SchedulesJob: true}, AnyValues: true, RequiresJob: true}),
	entry("transition", Specification{Effects: Effects{ReadsEntity: true, MutatesEntity: true}, UsesEntity: true, EntityFields: true, AllowedValues: []string{"entity", "id", "message"}, RequiresID: true, Transition: true}),
	entry("update", Specification{Effects: Effects{ReadsEntity: true, MutatesEntity: true}, UsesEntity: true, EntityFields: true, AllowedValues: []string{"entity", "id", "message"}, RequiresID: true, ProtectLifecycleState: true}),
)

func entry(name string, specification Specification) registry.Entry[Specification] {
	return registry.Entry[Specification]{Name: name, Value: specification}
}

func cloneSpecification(specification Specification) Specification {
	specification.AllowedValues = append([]string{}, specification.AllowedValues...)
	return specification
}

func Lookup(name string) (Specification, bool) {
	return specifications.Lookup(name)
}

func Names() []string {
	return specifications.Names()
}
