package appir

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/rule"
)

const (
	LegacyFormat      = "bean/appir/v1"
	LifecycleFormat   = "bean/appir/v2"
	RuleFormat        = "bean/appir/v3"
	TestSuiteFormat   = "bean/appir/v4"
	ExtensionFormat   = "bean/appir/v5"
	DisplayFormat     = "bean/appir/v6"
	ExploreFormat     = "bean/appir/v7"
	SequenceFormat    = "bean/appir/v8"
	InlinePanelFormat = "bean/appir/v9"
	PageSectionFormat = "bean/appir/v10"
	CurrentFormat     = "bean/appir/v11"
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
type DisplayTitle struct {
	Text, Field, Fallback string
}
type ViewColumn struct {
	Field, Label, LinkRoute string
}
type ViewControl struct {
	Filter, Label, Widget string
	Default               any
}
type ViewPager struct {
	Type     string
	PageSize int
}
type ViewRenderer struct {
	Type, TitleField, BodyField, LinkRoute, LinkField, EmptyState string
	GroupField, OrderField, ParentField, MoveAction               string
	MetricField, MetricLabel, TimeField, EndField                 string
	MetaFields, RichTextFields, Columns, SearchFields             []string
	Fields                                                        []ViewColumn
}
type Display struct {
	Type, Route, Description, EmptyState string
	Selection                            string
	Actions                              []string
	Title                                DisplayTitle
	Bindings                             map[string]ContextBinding
	Renderer                             ViewRenderer
	Controls                             []ViewControl
	Pager                                ViewPager
	Drill                                *ViewDrill
}
type ViewDrill struct {
	View, Display, Route string
	Bindings             []ViewDrillBinding
}
type ViewDrillBinding struct{ Source, Name, Filter string }
type FilterStep struct{ Type string }
type Filter struct {
	Name  string
	Steps []FilterStep
}
type ViewSearch struct {
	Fields []string
}
type ViewGroup struct {
	Field, As, Bucket string
}

func (g *ViewGroup) UnmarshalJSON(data []byte) error {
	var field string
	if err := json.Unmarshal(data, &field); err == nil {
		g.Field = field
		return nil
	}
	type encoded ViewGroup
	var value encoded
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*g = ViewGroup(value)
	return nil
}

func (g ViewGroup) Output() string {
	if g.As != "" {
		return g.As
	}
	return g.Field
}

type View struct {
	Name, Entity           string
	ResultShape            string
	Fields                 []string
	Relationships          []ViewRelationship
	Filter                 *expr.Expr
	ContextFilter          *expr.Expr
	ExposedFilters         map[string]ViewFilter
	Search                 ViewSearch
	FieldFilters           map[string]string
	Sort                   []Sort
	GroupBy                []ViewGroup
	Aggregates             []Aggregate
	Policy                 string
	DefaultLimit, MaxLimit int
	Displays               map[string]Display
}
type ViewFilter struct {
	Field, Operator             string
	Name, Label, Type           string
	Required, Unique, Sensitive bool
	Options                     []string
	Relation                    *Relation
}

func (f ViewFilter) Target(name string) string {
	if f.Field != "" {
		return f.Field
	}
	if f.Name != "" {
		return f.Name
	}
	return name
}

func (f ViewFilter) Definition(name string) Field {
	return Field{Name: f.Target(name), Label: f.Label, Type: f.Type, Required: f.Required, Unique: f.Unique, Sensitive: f.Sensitive, Options: f.Options, Relation: f.Relation}
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
	Name, Type, View, Display, Entity, Webform, Action, Menu, Text, Policy, Resource string
	Inputs                                                                           map[string]Field
	Bindings                                                                         map[string]ContextBinding
	Filters                                                                          []string
	DefaultFilters                                                                   map[string]any
	Presentation                                                                     ViewPresentation
	Content                                                                          []ContentElement
}
type ContentElement struct {
	Type, Text, Attribution, Source, Alt, Language, Tone, Direction string
	Items                                                           []string
}
type ViewPresentation struct {
	Mode, TitleField, BodyField, LinkRoute, LinkField, EmptyState string
	GroupField, OrderField, ParentField, MoveAction               string
	MetricField, MetricLabel, TimeField, EndField                 string
	MetaFields, RichTextFields, Columns, SearchFields             []string
}

func (r ViewRenderer) Presentation() ViewPresentation {
	return ViewPresentation{
		Mode: r.Type, TitleField: r.TitleField, BodyField: r.BodyField, LinkRoute: r.LinkRoute, LinkField: r.LinkField, EmptyState: r.EmptyState,
		GroupField: r.GroupField, OrderField: r.OrderField, ParentField: r.ParentField, MoveAction: r.MoveAction,
		MetricField: r.MetricField, MetricLabel: r.MetricLabel, TimeField: r.TimeField, EndField: r.EndField,
		MetaFields: r.MetaFields, RichTextFields: r.RichTextFields, Columns: r.Columns, SearchFields: r.SearchFields,
	}
}

func RendererFromPresentation(p ViewPresentation) ViewRenderer {
	return ViewRenderer{
		Type: p.Mode, TitleField: p.TitleField, BodyField: p.BodyField, LinkRoute: p.LinkRoute, LinkField: p.LinkField, EmptyState: p.EmptyState,
		GroupField: p.GroupField, OrderField: p.OrderField, ParentField: p.ParentField, MoveAction: p.MoveAction,
		MetricField: p.MetricField, MetricLabel: p.MetricLabel, TimeField: p.TimeField, EndField: p.EndField,
		MetaFields: p.MetaFields, RichTextFields: p.RichTextFields, Columns: p.Columns, SearchFields: p.SearchFields,
	}
}

type RegionItem struct {
	ID       string `json:"id"`
	Identity string
	Block    string
	Content  []ContentElement
}

type Region struct {
	Name              string
	CollapseWhenEmpty bool
	Blocks            []string
	Items             []RegionItem
}

// OrderedItems returns the canonical ordered region composition while keeping
// legacy blocks-only AppIR readable without rewriting it.
func (r Region) OrderedItems() []RegionItem {
	if r.Items != nil {
		return r.Items
	}
	items := make([]RegionItem, len(r.Blocks))
	for index, name := range r.Blocks {
		items[index] = RegionItem{Block: name}
	}
	return items
}

// ResolveBlock returns the existing named Block or the compiler-lowered
// content Block represented by a nested region item.
func (i RegionItem) ResolveBlock(app *App) (Block, bool) {
	if i.Block != "" {
		block, exists := app.Blocks[i.Block]
		return block, exists
	}
	if i.Identity == "" || i.Content == nil {
		return Block{}, false
	}
	return Block{Name: i.Identity, Type: "content", Content: i.Content}, true
}

type Panel struct {
	Name, Layout, Policy string
	Regions              []Region
}
type PageSection struct {
	ID       string `json:"id"`
	Panel    string
	Identity string
}

type Page struct {
	Name, Route, Title, Policy, Panel, Description string
	Sections                                       []PageSection
	Context                                        map[string]ContextBinding
	Filters                                        map[string]PageFilter
}

// OrderedSections returns Page sections in render order while preserving the
// legacy single-Panel representation in stored AppIR.
func (p Page) OrderedSections() []PageSection {
	if p.Sections != nil {
		return append([]PageSection(nil), p.Sections...)
	}
	if p.Panel != "" {
		return []PageSection{{Panel: p.Panel}}
	}
	return []PageSection{}
}

func (p Page) PanelNames() []string {
	sections := p.OrderedSections()
	panels := make([]string, len(sections))
	for index, section := range sections {
		panels[index] = section.Panel
	}
	return panels
}

type SequenceFrame struct {
	Name, Title, Layout, Panel, Notes string
}
type Sequence struct {
	Name, Route, Title, Description, Profile, AspectRatio, Policy string
	Frames                                                        []SequenceFrame
}
type PageFilter struct {
	Label, Type, Widget string
	Default             any
	Options             []string
	Targets             []PageFilterTarget
}
type PageFilterTarget struct{ Block, Filter string }
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
	Sequences         map[string]Sequence
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
	return &App{FormatVersion: CurrentFormat, Entities: map[string]Entity{}, Views: map[string]View{}, Actions: map[string]Action{}, Lifecycles: map[string]Lifecycle{}, Rules: map[string]Rule{}, TestSuites: map[string]TestSuite{}, Extensions: map[string]Extension{}, Policies: map[string]Policy{}, Webforms: map[string]Webform{}, Blocks: map[string]Block{}, Panels: map[string]Panel{}, Pages: map[string]Page{}, Sequences: map[string]Sequence{}, Roles: map[string]Role{}, Menus: map[string]Menu{}, Jobs: map[string]Job{}, Filters: map[string]Filter{}, AdminResources: map[string]AdminResource{}}
}
func (a *App) ValidateFormat() error {
	if a.FormatVersion != LegacyFormat && a.FormatVersion != LifecycleFormat && a.FormatVersion != RuleFormat && a.FormatVersion != TestSuiteFormat && a.FormatVersion != ExtensionFormat && a.FormatVersion != DisplayFormat && a.FormatVersion != ExploreFormat && a.FormatVersion != SequenceFormat && a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat {
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
	if a.FormatVersion != TestSuiteFormat && a.FormatVersion != ExtensionFormat && a.FormatVersion != DisplayFormat && a.FormatVersion != ExploreFormat && a.FormatVersion != SequenceFormat && a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat && len(a.TestSuites) > 0 {
		return fmt.Errorf("AppIR format %q cannot contain TestSuite definitions", a.FormatVersion)
	}
	if a.FormatVersion != ExtensionFormat && a.FormatVersion != DisplayFormat && a.FormatVersion != ExploreFormat && a.FormatVersion != SequenceFormat && a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat && len(a.Extensions) > 0 {
		return fmt.Errorf("AppIR format %q cannot contain Extension definitions", a.FormatVersion)
	}
	if a.FormatVersion != ExtensionFormat && a.FormatVersion != DisplayFormat && a.FormatVersion != ExploreFormat && a.FormatVersion != SequenceFormat && a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat {
		for _, action := range a.Actions {
			for _, step := range action.Steps {
				if step.Op == "extension" || step.Extension != "" {
					return fmt.Errorf("AppIR format %q cannot contain Extension-bound Actions", a.FormatVersion)
				}
			}
		}
		for _, suite := range a.TestSuites {
			for _, test := range suite.Tests {
				if len(test.Providers) > 0 || len(test.Expect.ProviderCalls) > 0 {
					return fmt.Errorf("AppIR format %q cannot contain Extension-bound TestSuites", a.FormatVersion)
				}
			}
		}
	}
	if a.FormatVersion != DisplayFormat && a.FormatVersion != ExploreFormat && a.FormatVersion != SequenceFormat && a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat {
		for _, view := range a.Views {
			for _, filter := range view.ExposedFilters {
				if filter.Field != "" || filter.Operator != "" {
					return fmt.Errorf("AppIR format %q cannot contain first-class View filters", a.FormatVersion)
				}
			}
			for _, display := range view.Displays {
				if display.Type == "page" || display.Type == "block" || display.Description != "" || display.EmptyState != "" ||
					display.Title != (DisplayTitle{}) || len(display.Bindings) > 0 || display.Renderer.Type != "" ||
					len(display.Controls) > 0 || display.Pager != (ViewPager{}) {
					return fmt.Errorf("AppIR format %q cannot contain first-class View displays", a.FormatVersion)
				}
			}
		}
		for _, block := range a.Blocks {
			if block.Display != "" {
				return fmt.Errorf("AppIR format %q cannot contain View display-bound Blocks", a.FormatVersion)
			}
		}
	}
	if a.FormatVersion != ExploreFormat && a.FormatVersion != SequenceFormat && a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat {
		for _, view := range a.Views {
			if len(view.Search.Fields) > 0 {
				return fmt.Errorf("AppIR format %q cannot contain View-owned search semantics", a.FormatVersion)
			}
			if view.ResultShape != "" {
				return fmt.Errorf("AppIR format %q cannot contain explicit View result shapes", a.FormatVersion)
			}
			for _, group := range view.GroupBy {
				if group.As != "" || group.Bucket != "" {
					return fmt.Errorf("AppIR format %q cannot contain typed View grouping semantics", a.FormatVersion)
				}
			}
			for _, display := range view.Displays {
				if display.Selection != "" || len(display.Actions) > 0 || display.Drill != nil {
					return fmt.Errorf("AppIR format %q cannot contain Explore interactions", a.FormatVersion)
				}
			}
		}
	}
	if a.FormatVersion != SequenceFormat && a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat {
		if len(a.Sequences) > 0 {
			return fmt.Errorf("AppIR format %q cannot contain Sequence definitions", a.FormatVersion)
		}
		for _, block := range a.Blocks {
			if len(block.Content) > 0 {
				return fmt.Errorf("AppIR format %q cannot contain semantic content Blocks", a.FormatVersion)
			}
		}
	}
	if a.FormatVersion != InlinePanelFormat && a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat {
		for _, panel := range a.Panels {
			for _, region := range panel.Regions {
				if region.Items != nil {
					return fmt.Errorf("AppIR format %q cannot contain ordered Panel region items", a.FormatVersion)
				}
			}
		}
	}
	if a.FormatVersion != PageSectionFormat && a.FormatVersion != CurrentFormat {
		for _, page := range a.Pages {
			if page.Sections != nil {
				return fmt.Errorf("AppIR format %q cannot contain ordered Page sections", a.FormatVersion)
			}
		}
	}
	if a.FormatVersion != CurrentFormat {
		for _, panel := range a.Panels {
			for _, region := range panel.Regions {
				if region.CollapseWhenEmpty {
					return fmt.Errorf("AppIR format %q cannot contain collapsible Panel Regions", a.FormatVersion)
				}
			}
		}
	}
	return nil
}
func (a *App) Clone() (*App, error) {
	b, e := json.Marshal(a)
	if e != nil {
		return nil, e
	}
	return Decode(b)
}

func Decode(encoded []byte) (*App, error) {
	var out App
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	err := decoder.Decode(&out)
	return &out, err
}
