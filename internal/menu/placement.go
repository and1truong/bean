package menu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/uid"
	"github.com/beanruntime/bean/internal/view"
)

const PlacementTable = "bean_menu_placement"
const ActionInputKey = "_navigation"

type PlacementInput struct {
	Menu          string `json:"menu"`
	OwnerID       string `json:"ownerId"`
	ParentID      string `json:"parentId,omitempty"`
	Weight        int    `json:"weight"`
	LabelOverride string `json:"labelOverride,omitempty"`
}

type Submission struct {
	Placements []PlacementInput `json:"placements"`
}

func DecodeSubmission(value any) (Submission, error) {
	var encoded []byte
	var err error
	if text, ok := value.(string); ok {
		encoded = []byte(text)
	} else {
		encoded, err = json.Marshal(value)
	}
	if err != nil {
		return Submission{}, invalid("navigation input is invalid")
	}
	var submission Submission
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&submission); err != nil {
		return Submission{}, invalid("navigation input is invalid")
	}
	if submission.Placements == nil {
		return Submission{}, invalid("navigation placements are required")
	}
	return submission, nil
}

func invalid(message string) error  { return &dbal.Error{Code: dbal.InvalidQuery, Message: message} }
func conflict(message string) error { return &dbal.Error{Code: dbal.Conflict, Message: message} }

// ReplaceTargetPlacements validates and replaces the complete dynamic placement
// set for one Entity record inside the caller's Action transaction.
func ReplaceTargetPlacements(ctx context.Context, tx dbal.Transaction, app *appir.App, entityName, targetID string, submission Submission, request beanctx.Request, now string) (bool, error) {
	entity := app.Entities[entityName]
	if entity.Navigation == nil {
		return false, invalid("Entity does not allow navigation placements")
	}
	if len(submission.Placements) > MaxEditorInstances {
		return false, invalid(fmt.Sprintf("navigation accepts at most %d placements", MaxEditorInstances))
	}
	eligible := map[string]bool{}
	for _, name := range entity.Navigation.Menus {
		eligible[name] = true
	}
	desired := map[string]PlacementInput{}
	for _, input := range submission.Placements {
		if !eligible[input.Menu] {
			return false, invalid("navigation Menu is not allowed for this Entity")
		}
		menuDefinition, exists := app.Menus[input.Menu]
		if !exists || menuDefinition.Owner == nil {
			return false, invalid("navigation Menu is not owner-scoped")
		}
		if input.OwnerID == "" {
			return false, invalid("navigation ownerId is required")
		}
		key := input.Menu + "\x00" + input.OwnerID
		if _, duplicate := desired[key]; duplicate {
			return false, conflict("target already has a placement in this Menu instance")
		}
		if input.Weight < MinWeight || input.Weight > MaxWeight {
			return false, invalid(fmt.Sprintf("navigation weight must be between %d and %d", MinWeight, MaxWeight))
		}
		if utf8.RuneCountInString(input.LabelOverride) > MaxLabelOverrideLength {
			return false, invalid(fmt.Sprintf("navigation labelOverride must contain at most %d characters", MaxLabelOverrideLength))
		}
		if _, err := view.ReadEntityRecord(ctx, tx, app, menuDefinition.Owner.Entity, input.OwnerID, request); err != nil {
			return false, &dbal.Error{Code: dbal.NotFound, Message: "navigation Menu owner not found", Cause: err}
		}
		desired[key] = input
	}
	targetPredicate := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "target_entity", Value: entityName}, dbal.Predicate{Op: dbal.OpEQ, Column: "target_id", Value: targetID})
	existing, err := tx.Select(ctx, dbal.Select{Table: PlacementTable, Where: &targetPredicate, Limit: MaxEditorInstances})
	if err != nil {
		return false, err
	}
	byKey := map[string]dbal.Row{}
	for _, row := range existing {
		byKey[fmt.Sprint(row["menu_name"])+"\x00"+fmt.Sprint(row["owner_id"])] = row
	}
	changed := false
	for key, row := range byKey {
		if _, keep := desired[key]; keep {
			continue
		}
		if err = authorizeExistingOwner(ctx, tx, app, row, request); err != nil {
			return false, err
		}
		children, selectErr := tx.Select(ctx, dbal.Select{Table: PlacementTable, Columns: []string{"id"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "parent_id", Value: fmt.Sprint(row["id"])}, Limit: 1})
		if selectErr != nil {
			return false, selectErr
		}
		if len(children) > 0 {
			return false, conflict("navigation placement has children")
		}
		if _, err = tx.Delete(ctx, dbal.Delete{Table: PlacementTable, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: fmt.Sprint(row["id"])}, ExpectedRows: 1}); err != nil {
			return false, err
		}
		changed = true
	}
	for key, input := range desired {
		row, exists := byKey[key]
		placementID := ""
		if exists {
			placementID = fmt.Sprint(row["id"])
		} else {
			placementID = uid.New()
		}
		if err = validateParentAndDepth(ctx, tx, input, placementID); err != nil {
			return false, err
		}
		if exists {
			values := map[string]dbal.Value{"parent_id": nullable(input.ParentID), "weight": input.Weight, "label_override": nullable(input.LabelOverride), "updated_at": now}
			if samePlacement(row, input) {
				continue
			}
			if _, err = tx.Update(ctx, dbal.Update{Table: PlacementTable, Values: values, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: placementID}, ExpectedRows: 1}); err != nil {
				return false, err
			}
			changed = true
			continue
		}
		count, countErr := tx.Select(ctx, dbal.Select{Table: PlacementTable, Columns: []string{"id"}, Where: instancePredicate(input.Menu, input.OwnerID), Limit: MaxPlacements + 1})
		if countErr != nil {
			return false, countErr
		}
		if len(count) >= MaxPlacements {
			return false, conflict(fmt.Sprintf("navigation Menu instance may contain at most %d placements", MaxPlacements))
		}
		_, err = tx.Insert(ctx, dbal.Insert{Table: PlacementTable, Values: map[string]dbal.Value{
			"id": placementID, "menu_name": input.Menu, "owner_entity": app.Menus[input.Menu].Owner.Entity, "owner_id": input.OwnerID,
			"target_entity": entityName, "target_id": targetID, "parent_id": nullable(input.ParentID), "weight": input.Weight,
			"label_override": nullable(input.LabelOverride), "created_at": now, "updated_at": now,
		}})
		if err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func validateParentAndDepth(ctx context.Context, tx dbal.Transaction, input PlacementInput, placementID string) error {
	if input.ParentID == "" {
		return nil
	}
	rows, err := tx.Select(ctx, dbal.Select{Table: PlacementTable, Where: instancePredicate(input.Menu, input.OwnerID), Limit: MaxPlacements})
	if err != nil {
		return err
	}
	parents := map[string]string{}
	found := false
	for _, row := range rows {
		id := fmt.Sprint(row["id"])
		parents[id] = stringValue(row["parent_id"])
		found = found || id == input.ParentID
	}
	if !found {
		return invalid("navigation parent is not in this Menu instance")
	}
	parents[placementID] = input.ParentID
	seen, current := map[string]bool{}, placementID
	for depth := 1; current != ""; depth++ {
		if seen[current] {
			return conflict("navigation placement creates a cycle")
		}
		if depth > MaxDepth {
			return conflict(fmt.Sprintf("navigation placement exceeds maximum depth %d", MaxDepth))
		}
		seen[current] = true
		current = parents[current]
	}
	return nil
}

func authorizeExistingOwner(ctx context.Context, tx dbal.Transaction, app *appir.App, row dbal.Row, request beanctx.Request) error {
	menuDefinition, exists := app.Menus[fmt.Sprint(row["menu_name"])]
	if !exists || menuDefinition.Owner == nil || menuDefinition.Owner.Entity != fmt.Sprint(row["owner_entity"]) {
		return conflict("navigation placement references an unavailable Menu contract")
	}
	_, err := view.ReadEntityRecord(ctx, tx, app, menuDefinition.Owner.Entity, fmt.Sprint(row["owner_id"]), request)
	if err != nil {
		return &dbal.Error{Code: dbal.NotFound, Message: "navigation Menu owner not found", Cause: err}
	}
	return nil
}

func instancePredicate(menuName, ownerID string) *dbal.Predicate {
	predicate := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "menu_name", Value: menuName},
		dbal.Predicate{Op: dbal.OpEQ, Column: "owner_id", Value: ownerID},
	)
	return &predicate
}

func nullable(value string) dbal.Value {
	if value == "" {
		return nil
	}
	return value
}

func samePlacement(row dbal.Row, input PlacementInput) bool {
	return stringValue(row["parent_id"]) == input.ParentID && intValue(row["weight"]) == input.Weight && stringValue(row["label_override"]) == input.LabelOverride
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int64:
		return int(typed)
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}

// DeleteRecordPlacements applies target and owner cleanup inside a delete Action.
// Target placements with children reject deletion unless their entire owner Menu
// instance is being removed by the same owner deletion.
func DeleteRecordPlacements(ctx context.Context, tx dbal.Transaction, app *appir.App, entityName, recordID string) (bool, error) {
	changed := false
	ownerInstances := map[string]bool{}
	for name, definition := range app.Menus {
		if definition.Owner != nil && definition.Owner.Entity == entityName {
			ownerInstances[name+"\x00"+recordID] = true
			result, err := tx.Delete(ctx, dbal.Delete{Table: PlacementTable, Where: *instancePredicate(name, recordID)})
			if err != nil {
				return false, err
			}
			changed = changed || result.Affected > 0
		}
	}
	targetPredicate := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "target_entity", Value: entityName}, dbal.Predicate{Op: dbal.OpEQ, Column: "target_id", Value: recordID})
	rows, err := tx.Select(ctx, dbal.Select{Table: PlacementTable, Where: &targetPredicate, Limit: MaxEditorInstances})
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if ownerInstances[fmt.Sprint(row["menu_name"])+"\x00"+fmt.Sprint(row["owner_id"])] {
			continue
		}
		children, childErr := tx.Select(ctx, dbal.Select{Table: PlacementTable, Columns: []string{"id"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "parent_id", Value: fmt.Sprint(row["id"])}, Limit: 1})
		if childErr != nil {
			return false, childErr
		}
		if len(children) > 0 {
			return false, conflict("navigation placement has children")
		}
		if _, err = tx.Delete(ctx, dbal.Delete{Table: PlacementTable, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: fmt.Sprint(row["id"])}, ExpectedRows: 1}); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}
