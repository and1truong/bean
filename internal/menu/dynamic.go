package menu

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/view"
)

func DynamicTree(ctx context.Context, reader view.Reader, app *appir.App, menuName, ownerID string, request beanctx.Request) ([]RenderItem, error) {
	definition, exists := app.Menus[menuName]
	if !exists || definition.Owner == nil {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "Menu not found"}
	}
	if ownerID == "" {
		return nil, invalid("Menu owner is required")
	}
	if _, err := view.ReadEntityRecord(ctx, reader, app, definition.Owner.Entity, ownerID, request); err != nil {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "Menu owner not found", Cause: err}
	}
	rows, err := reader.Select(ctx, dbal.Select{Table: PlacementTable, Where: instancePredicate(menuName, ownerID), OrderBy: []dbal.Order{{Column: "weight"}, {Column: "id"}}, Limit: MaxPlacements + 1})
	if err != nil {
		return nil, err
	}
	if len(rows) > MaxPlacements {
		return nil, &dbal.Error{Code: dbal.ResultLimitExceeded, Message: fmt.Sprintf("Menu instance exceeds %d placements", MaxPlacements)}
	}
	visible := map[string]appir.MenuItem{}
	for _, row := range rows {
		entityName, targetID := fmt.Sprint(row["target_entity"]), fmt.Sprint(row["target_id"])
		entity, entityExists := app.Entities[entityName]
		if !entityExists || entity.Navigation == nil || !contains(entity.Navigation.Menus, menuName) {
			continue
		}
		viewDefinition, viewExists := app.Views[entity.Navigation.Destination.View]
		display, displayExists := viewDefinition.Displays[entity.Navigation.Destination.Display]
		if !viewExists || !displayExists || display.Type != "page" {
			continue
		}
		result, readErr := view.ReadPage(ctx, reader, app, viewDefinition.Name, view.ReadOptions{Params: view.Params{RecordID: targetID, Limit: 1}}, request)
		if readErr != nil || len(result.Rows) == 0 {
			continue
		}
		target := result.Rows[0]
		label := stringValue(row["label_override"])
		if label == "" {
			label = stringValue(target[entity.Navigation.LabelField])
		}
		route, routeAvailable := bindRecordRoute(display.Route, target, menuName, ownerID)
		if label == "" || !routeAvailable {
			continue
		}
		visible[fmt.Sprint(row["id"])] = appir.MenuItem{
			ID: fmt.Sprint(row["id"]), Label: label, Route: route, Parent: stringValue(row["parent_id"]), Weight: intValue(row["weight"]),
		}
	}
	children := map[string][]appir.MenuItem{}
	for _, item := range visible {
		if item.Parent != "" {
			if _, parentVisible := visible[item.Parent]; !parentVisible {
				continue
			}
		}
		children[item.Parent] = append(children[item.Parent], item)
	}
	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool {
			if children[parent][i].Weight != children[parent][j].Weight {
				return children[parent][i].Weight < children[parent][j].Weight
			}
			return children[parent][i].ID < children[parent][j].ID
		})
	}
	return renderChildren(children, "", request.Route, 1), nil
}

func bindRecordRoute(route string, record dbal.Row, menuName, ownerID string) (string, bool) {
	parts := strings.Split(route, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, ":") {
			value := stringValue(record[strings.TrimPrefix(part, ":")])
			if value == "" {
				return "", false
			}
			parts[index] = url.PathEscape(value)
		}
	}
	resolved := strings.Join(parts, "/")
	query := url.Values{"_menu": []string{menuName}, "_owner": []string{ownerID}}
	return resolved + "?" + query.Encode(), true
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
