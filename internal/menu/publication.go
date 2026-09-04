package menu

import (
	"context"
	"fmt"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal"
)

type placementReader interface {
	Select(context.Context, dbal.Select) ([]dbal.Row, error)
}

// ValidatePublication prevents immutable navigation contracts from orphaning
// persisted dynamic placements. It never mutates placement data.
func ValidatePublication(ctx context.Context, reader placementReader, current, next *appir.App) error {
	if current == nil {
		return nil
	}
	for name, oldMenu := range current.Menus {
		if oldMenu.Owner == nil {
			continue
		}
		newMenu, exists := next.Menus[name]
		if exists && newMenu.Owner != nil && newMenu.Owner.Entity == oldMenu.Owner.Entity {
			continue
		}
		found, err := hasPlacement(ctx, reader, &dbal.Predicate{Op: dbal.OpEQ, Column: "menu_name", Value: name})
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf("Menu %s cannot be removed or change owner while dynamic placements exist", name)
		}
	}
	for name, oldEntity := range current.Entities {
		if oldEntity.Navigation == nil {
			continue
		}
		newEntity, exists := next.Entities[name]
		if !exists || newEntity.Navigation == nil || oldEntity.Navigation.LabelField != newEntity.Navigation.LabelField || oldEntity.Navigation.Destination != newEntity.Navigation.Destination {
			found, err := hasPlacement(ctx, reader, &dbal.Predicate{Op: dbal.OpEQ, Column: "target_entity", Value: name})
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("Entity %s navigation destination cannot be removed or changed while dynamic placements exist", name)
			}
			continue
		}
		nextMenus := map[string]bool{}
		for _, menuName := range newEntity.Navigation.Menus {
			nextMenus[menuName] = true
		}
		for _, removedMenu := range oldEntity.Navigation.Menus {
			if nextMenus[removedMenu] {
				continue
			}
			where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "target_entity", Value: name}, dbal.Predicate{Op: dbal.OpEQ, Column: "menu_name", Value: removedMenu})
			found, err := hasPlacement(ctx, reader, &where)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("Entity %s cannot leave Menu %s while dynamic placements exist", name, removedMenu)
			}
		}
	}
	return nil
}

func hasPlacement(ctx context.Context, reader placementReader, where *dbal.Predicate) (bool, error) {
	rows, err := reader.Select(ctx, dbal.Select{Table: PlacementTable, Columns: []string{"id"}, Where: where, Limit: 1})
	return len(rows) > 0, err
}
