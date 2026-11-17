package catalog

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
	"reflect"
)

func PrepareRepair(ctx context.Context, namespace string, source workflow.Definition, object string, revision int) (*Execution, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	item, err := lockedObject(ctx, tx, namespace, object, revision)
	if err != nil {
		return nil, err
	}
	if item.Kind != "workflow" || item.Spec.Flow == nil {
		return nil, ErrInvalid
	}
	base, err := readVersion(ctx, tx, namespace, source.VersionID)
	if err != nil {
		return nil, err
	}
	if base.Kind != "workflow" || base.Spec.Flow == nil {
		return nil, ErrInvalid
	}
	next, err := compileFlow(ctx, tx, namespace, source.ProjectID, item.Spec.Flow, true)
	if err != nil {
		return nil, err
	}
	next.Input, next.VersionID = source.Input, source.VersionID
	if err = next.ValidateInput(); err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(next.Budget, source.Budget) || !reflect.DeepEqual(next.InputSchema, source.InputSchema) {
		return nil, invalid("定向修正必须保留原输入契约与调用预算")
	}
	changes := repairChanges(base.Spec.Flow, item.Spec.Flow)
	if len(changes) == 0 {
		return nil, invalid("修正草稿没有执行图差异")
	}
	return &Execution{BaseVersion: source.VersionID, ObjectID: object, Revision: revision, Flow: item.Spec.Flow, Workflow: *next, Changes: changes, Assignments: []workflow.TaskAssignment{}}, nil
}

func repairChanges(before, after *Flow) []GraphChange {
	changes := []GraphChange{}
	old, next := map[string]Binding{}, map[string]Binding{}
	for _, n := range before.Nodes {
		old[n.ID] = n
	}
	for _, n := range after.Nodes {
		next[n.ID] = n
	}
	for _, n := range before.Nodes {
		v, exists := next[n.ID]
		if !exists {
			changes = append(changes, GraphChange{NodeID: n.ID, Before: &n})
		} else if !reflect.DeepEqual(n, v) {
			changes = append(changes, GraphChange{NodeID: n.ID, Before: &n, After: &v})
		}
	}
	for _, n := range after.Nodes {
		if _, exists := old[n.ID]; !exists {
			changes = append(changes, GraphChange{NodeID: n.ID, After: &n})
		}
	}
	return changes
}

func CheckRepairDraft(ctx context.Context, tx pgx.Tx, namespace string, p *Execution) error {
	item, err := lockedObject(ctx, tx, namespace, p.ObjectID, p.Revision)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(item.Spec.Flow)
	b, _ := json.Marshal(p.Flow)
	if item.Kind != "workflow" || string(a) != string(b) {
		return ErrConflict
	}
	return nil
}
