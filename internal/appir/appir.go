package appir

import (
	"encoding/json"
	"fmt"

	"github.com/beanruntime/bean/internal/expr"
)

const CurrentFormat = "bean/appir/v1"

type Field struct {
	Name, Label, Type string
	Required, Unique  bool
	Options           []string
	Relation          *Relation
}
type Relation struct{ Entity, Kind, TargetField string }
type Entity struct {
	Name, Label, Policy       string
	Fields                    []Field
	Owner, Tenant, SoftDelete bool
	Indexes, Unique           [][]string
}
type Display struct{ Type, Route string }
type View struct {
	Name, Entity           string
	Fields                 []string
	Relationships          []ViewRelationship
	Filter                 *expr.Expr
	ContextFilter          *expr.Expr
	ExposedFilters         map[string]Field
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
	StateField                      string
	Input                           map[string]Field
	Output                          map[string]Field
	Steps                           []Step
	Transitions                     map[string][]string
}
type Step struct {
	Op, Result string
	Entity     string
	View       string
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
	Name, Type, View, Entity, Webform, Action, Menu, Text, Policy string
	Inputs                                                        map[string]Field
	Bindings                                                      map[string]ContextBinding
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
	ReleaseID, AppID string
	FormatVersion    string
	Version          int
	Entities         map[string]Entity
	Views            map[string]View
	Actions          map[string]Action
	Policies         map[string]Policy
	Webforms         map[string]Webform
	Blocks           map[string]Block
	Panels           map[string]Panel
	Pages            map[string]Page
	Roles            map[string]Role
	Menus            map[string]Menu
	Jobs             map[string]Job
	AdminResources   map[string]AdminResource
	OpenAPI          json.RawMessage
}

func Empty() *App {
	return &App{FormatVersion: CurrentFormat, Entities: map[string]Entity{}, Views: map[string]View{}, Actions: map[string]Action{}, Policies: map[string]Policy{}, Webforms: map[string]Webform{}, Blocks: map[string]Block{}, Panels: map[string]Panel{}, Pages: map[string]Page{}, Roles: map[string]Role{}, Menus: map[string]Menu{}, Jobs: map[string]Job{}, AdminResources: map[string]AdminResource{}}
}
func (a *App) ValidateFormat() error {
	if a.FormatVersion != CurrentFormat {
		return fmt.Errorf("unsupported AppIR format %q", a.FormatVersion)
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
