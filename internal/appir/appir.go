package appir

import (
	"encoding/json"
	"fmt"

	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/rule"
)

const (
	LegacyFormat    = "bean/appir/v1"
	LifecycleFormat = "bean/appir/v2"
	RuleFormat      = "bean/appir/v3"
	TestSuiteFormat = "bean/appir/v4"
	CurrentFormat   = "bean/appir/v5"
)

type Field struct {
	Name, Label, Type string
	Required, Unique  bool
	Sensitive         bool
	Options           []string
	Relation          *Relation
}
type Relation struct{ Entity, Kind, TargetField string }
type Entity struct {
	Name, Label, Policy       string
	Fields                    []Field
	Owner, Tenant, SoftDelete bool
	Indexes, Unique           [][]string
	Validations               map[string]string
}
type Display struct{ Type, Route string }
type FilterStep struct{ Type string }
type Filter struct {
	Name  string
	Steps []FilterStep
}
type View struct {
	Name, Entity           string
	Fields                 []string
	Relationships          []ViewRelationship
	Filter                 *expr.Expr
	ContextFilter          *expr.Expr
	ExposedFilters         map[string]Field
	FieldFilters           map[string]string
	Sort                   []Sort
	GroupBy                []string
	Aggregates             []Aggregate
	Policy                 string
	DefaultLimit, MaxLimit int
	Displays               map[string]Display
}
type ViewRelationship struct {
	Name, Entity, Type, LocalField, TargetField, RelationField string
}
type Aggregate struct{ Function, Field, Alias string }
type Sort struct {
	Field string
	Desc  bool
}
type Action struct {
	Name, Entity, Operation, Policy string
	Lifecycle                       string
	DefaultRole                     string
	Confirm                         string
	StateField                      string
	Input                           map[string]Field
	Output                          map[string]Field
	Steps                           []Step
	Transitions                     map[string][]string
	When                            string
	Derive                          map[string]string
}
type Rule struct {
	Name, Entity string
	Result       rule.Type
	Input        map[string]Field
	Expression   rule.Expression
}
type TestTarget struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}
type TestActor struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"displayName"`
	Roles       []string `json:"roles"`
}
type TestContext struct {
	Actor     *TestActor `json:"actor"`
	Tenant    string     `json:"tenant"`
	Time      string     `json:"time"`
	RequestID string     `json:"requestId"`
	IDs       []string   `json:"ids"`
	Seed      *int64     `json:"seed"`
}
type TestEvent struct {
	Topic   string         `json:"topic"`
	Payload map[string]any `json:"payload"`
}
type TestAudit struct {
	Action   string   `json:"action"`
	ActorID  string   `json:"actorId"`
	TenantID string   `json:"tenantId"`
	Entity   string   `json:"entity"`
	EntityID string   `json:"entityId"`
	Changed  []string `json:"changed"`
	Success  *bool    `json:"success"`
}
type TestMutation struct {
	Entity string         `json:"entity"`
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
	Absent bool           `json:"absent"`
}
type TestExpectation struct {
	Result        json.RawMessage    `json:"result"`
	Error         string             `json:"error"`
	Changes       []TestMutation     `json:"changes"`
	Events        []TestEvent        `json:"events"`
	Audit         []TestAudit        `json:"audit"`
	ProviderCalls []TestProviderCall `json:"providerCalls"`
	NoChanges     bool               `json:"noChanges"`
	NoEvents      bool               `json:"noEvents"`
}
type TestProviderResult struct {
	Output map[string]any `json:"output"`
	Error  string         `json:"error"`
}
type TestProviderCall struct {
	Extension      string         `json:"extension"`
	InvocationID   string         `json:"invocationId"`
	IdempotencyKey string         `json:"idempotencyKey"`
	Input          map[string]any `json:"input"`
}
type TestCase struct {
	Name      string                          `json:"name"`
	Fixtures  map[string][]map[string]any     `json:"fixtures"`
	Input     map[string]any                  `json:"input"`
	This      map[string]any                  `json:"this"`
	Context   TestContext                     `json:"context"`
	Providers map[string][]TestProviderResult `json:"providers"`
	Expect    TestExpectation                 `json:"expect"`
}
type TestSuite struct {
	Name   string     `json:"name"`
	Target TestTarget `json:"target"`
	Tests  []TestCase `json:"tests"`
}
type ExtensionRetry struct {
	MaxAttempts  int
	DelaySeconds int
}
type Extension struct {
	Name, Transport, Endpoint, Authentication string
	Idempotency, Transaction, Failure         string
	Input, Output                             map[string]Field
	Permissions, SideEffects                  []string
	TimeoutSeconds                            int
	Retry                                     ExtensionRetry
}
type Lifecycle struct {
	Name, Entity, StateField, Initial string
	Transitions                       map[string][]string
}
type LocalRegistration struct{ Action, Route string }
type Step struct {
	Op, Result string
	Entity     string
	View       string
	Extension  string
	StateField string
	Values     []Assignment
	Where      *expr.Expr
	Condition  *expr.Expr
	Event, Job string
}
type Assignment struct {
	Field string
	Value ValueBinding
}
type ValueBinding struct {
	Source, Path string
	Literal      json.RawMessage
}
type Policy struct {
	Name                  string
	ReadRoles, WriteRoles []string
	Authenticated         bool
	BypassOwnerRoles      []string
	Owner, Tenant         bool
	OwnerOrPublic         bool
	PublicField           string
	PublicValue           any
	Condition             *expr.Expr
	Redact                []string
}
type FormElement struct {
	Name, Type            string
	Required              bool
	Min, Max              *float64
	MinLength, MaxLength  int
	Pattern               string
	Options               []string
	Visible, RequiredWhen *expr.Expr
	Children              []FormElement
}
type Webform struct {
	Name, Action string
	Elements     []FormElement
	Steps        [][]string
	Confirmation string
}
type Block struct {
	Name, Type, View, Entity, Webform, Action, Menu, Text, Policy, Resource string
	Inputs                                                                  map[string]Field
	Bindings                                                                map[string]ContextBinding
	Filters                                                                 []string
	DefaultFilters                                                          map[string]any
	Presentation                                                            ViewPresentation
}
type ViewPresentation struct {
	Mode, TitleField, BodyField, LinkRoute, LinkField, EmptyState string
	GroupField, OrderField, ParentField, MoveAction               string
	MetricField, MetricLabel, TimeField                           string
	MetaFields, RichTextFields, Columns, SearchFields             []string
}
type Region struct {
	Name   string
	Blocks []string
}
type Panel struct {
	Name, Layout, Policy string
	Regions              []Region
}
type Page struct {
	Name, Route, Title, Policy, Panel, Description string
	Context                                        map[string]ContextBinding
}
type ContextBinding struct {
	Source, Name string
	Required     bool
}
type Role struct {
	Name        string
	Permissions []string
}
type Menu struct {
	Name  string
	Items []MenuItem
}
type MenuItem struct{ Label, Route, Policy string }
type Job struct{ Name, Action string }
type Theme struct {
	Name, DisplayName, Preset, Accent string
}
type DemoSeedEntity struct {
	Count   int
	Profile string
}
type DemoSeed struct {
	Name     string
	Entities map[string]DemoSeedEntity
}
type AdminList struct {
	Columns, Search, Filters []string
	Sort                     []Sort
	PageSize                 int
}
type AdminForm struct {
	Fields, Readonly []string
}
type AdminResource struct {
	Name, Entity, Label, Description, LabelField   string
	View, CreateAction, UpdateAction, DeleteAction string
	List                                           AdminList
	Form                                           AdminForm
	Actions                                        []string
}
type App struct {
	ReleaseID, AppID  string
	FormatVersion     string
	Version           int
	Entities          map[string]Entity
	Views             map[string]View
	Actions           map[string]Action
	Lifecycles        map[string]Lifecycle
	Rules             map[string]Rule
	TestSuites        map[string]TestSuite
	Extensions        map[string]Extension
	Policies          map[string]Policy
	Webforms          map[string]Webform
	Blocks            map[string]Block
	Panels            map[string]Panel
	Pages             map[string]Page
	Roles             map[string]Role
	Menus             map[string]Menu
	Jobs              map[string]Job
	Filters           map[string]Filter
	AdminResources    map[string]AdminResource
	LocalRegistration *LocalRegistration
	Theme             *Theme
	DemoSeed          *DemoSeed
	OpenAPI           json.RawMessage
}

func Empty() *App {
	return &App{FormatVersion: CurrentFormat, Entities: map[string]Entity{}, Views: map[string]View{}, Actions: map[string]Action{}, Lifecycles: map[string]Lifecycle{}, Rules: map[string]Rule{}, TestSuites: map[string]TestSuite{}, Extensions: map[string]Extension{}, Policies: map[string]Policy{}, Webforms: map[string]Webform{}, Blocks: map[string]Block{}, Panels: map[string]Panel{}, Pages: map[string]Page{}, Roles: map[string]Role{}, Menus: map[string]Menu{}, Jobs: map[string]Job{}, Filters: map[string]Filter{}, AdminResources: map[string]AdminResource{}}
}
func (a *App) ValidateFormat() error {
	if a.FormatVersion != LegacyFormat && a.FormatVersion != LifecycleFormat && a.FormatVersion != RuleFormat && a.FormatVersion != TestSuiteFormat && a.FormatVersion != CurrentFormat {
		return fmt.Errorf("unsupported AppIR format %q", a.FormatVersion)
	}
	if a.FormatVersion == LegacyFormat {
		if len(a.Lifecycles) > 0 {
			return fmt.Errorf("AppIR format %q cannot contain Lifecycle definitions", a.FormatVersion)
		}
		for _, action := range a.Actions {
			if action.Lifecycle != "" {
				return fmt.Errorf("AppIR format %q cannot contain Lifecycle-bound Actions", a.FormatVersion)
			}
		}
	}
	if a.FormatVersion == LegacyFormat || a.FormatVersion == LifecycleFormat {
		if len(a.Rules) > 0 {
			return fmt.Errorf("AppIR format %q cannot contain Rule definitions", a.FormatVersion)
		}
		for _, entity := range a.Entities {
			if len(entity.Validations) > 0 {
				return fmt.Errorf("AppIR format %q cannot contain Rule-bound Entity validations", a.FormatVersion)
			}
		}
		for _, action := range a.Actions {
			if action.When != "" || len(action.Derive) > 0 {
				return fmt.Errorf("AppIR format %q cannot contain Rule-bound Actions", a.FormatVersion)
			}
		}
	}
	if a.FormatVersion != TestSuiteFormat && a.FormatVersion != CurrentFormat && len(a.TestSuites) > 0 {
		return fmt.Errorf("AppIR format %q cannot contain TestSuite definitions", a.FormatVersion)
	}
	if a.FormatVersion != CurrentFormat && len(a.Extensions) > 0 {
		return fmt.Errorf("AppIR format %q cannot contain Extension definitions", a.FormatVersion)
	}
	return nil
}
func (a *App) Clone() (*App, error) {
	b, e := json.Marshal(a)
	if e != nil {
		return nil, e
	}
	var out App
	e = json.Unmarshal(b, &out)
	return &out, e
}
