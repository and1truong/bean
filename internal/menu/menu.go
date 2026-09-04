// Package menu owns bounded hierarchical navigation semantics.
package menu

import (
	"sort"

	"github.com/beanruntime/bean/internal/appir"
)

const (
	ProfileWorkspace       = "workspace"
	VariantDefault         = "default"
	VariantLine            = "line"
	MaxDefinitions         = 32
	MaxDepth               = 3
	MaxPlacements          = 200
	MaxEditorInstances     = 32
	MinWeight              = -1000
	MaxWeight              = 1000
	MaxLabelOverrideLength = 120
)

func Profiles() []string             { return []string{ProfileWorkspace} }
func Variants() []string             { return []string{VariantDefault, VariantLine} }
func ValidVariant(value string) bool { return value == VariantDefault || value == VariantLine }

func IsTypedTarget(target appir.MenuTarget) bool {
	return target.Page != "" || target.View != "" || target.Display != ""
}

func TargetKey(target appir.MenuTarget) string {
	if target.Page != "" {
		return "Page/" + target.Page
	}
	return "View/" + target.View + "/" + target.Display
}

// Ordered returns a stable sibling ordering without changing hierarchy.
func Ordered(items []appir.MenuItem) []appir.MenuItem {
	out := append([]appir.MenuItem(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Parent != out[j].Parent {
			return out[i].Parent < out[j].Parent
		}
		if out[i].Weight != out[j].Weight {
			return out[i].Weight < out[j].Weight
		}
		return out[i].ID < out[j].ID
	})
	return out
}
