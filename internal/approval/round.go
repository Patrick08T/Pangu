package approval

import (
	"context"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func lockActiveRun(ctx context.Context, tx pgx.Tx, run string) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", run); err != nil {
		return err
	}
	var cancelled bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1)", run).Scan(&cancelled); err != nil {
		return err
	}
	if cancelled {
		return ErrConflict
	}
	return nil
}

func inputReview(input workflow.NodeInput) (*workflow.Review, error) {
	if input.ApprovalRound < 1 || input.Node.Approval == nil || input.Node.Approval.SourceNode == "" {
		return nil, ErrInvalid
	}
	parent, ok := input.Parents[input.Node.Approval.SourceNode]
	if !ok {
		return nil, fmt.Errorf("%w: 缺少审核来源", ErrInvalid)
	}
	review, err := workflow.ParseReview(parent.Text)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return review, nil
}

func previousRound(ctx context.Context, tx pgx.Tx, namespace, run string, input workflow.NodeInput, review *workflow.Review) (string, error) {
	if input.ApprovalRound == 1 {
		if len(review.Resolutions) > 0 {
			var inherited bool
			err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.plan_continuations c JOIN pangu.submissions s ON s.id=c.request_id WHERE s.namespace=$1 AND s.run_id=$2)`, namespace, run).Scan(&inherited)
			if err != nil {
				return "", err
			}
			if !inherited {
				return "", ErrInvalid
			}
		}
		return "", nil
	}
	prior, err := scan(tx.QueryRow(ctx, "SELECT "+columns+" FROM pangu.approvals a JOIN pangu.submissions s ON s.id=a.request_id WHERE a.namespace=$1 AND a.run_id=$2 AND a.node_id=$3 AND a.round=$4 AND a.approval_generation=$5 AND a.invalidated_at IS NULL FOR UPDATE OF a", namespace, run, input.Node.ID, input.ApprovalRound-1, input.ApprovalGeneration))
	if err != nil {
		return "", err
	}
	if prior.Status != "changes_requested" || prior.Review == nil {
		return "", ErrConflict
	}
	required := map[string]bool{}
	known := map[string]bool{"overall": true}
	for _, item := range prior.Review.Items {
		known[item.ID] = true
	}
	for _, r := range prior.Review.Resolutions {
		known[r.ItemID] = true
		if r.Status == "unresolved" {
			required[r.ItemID] = true
		}
	}
	for _, answer := range prior.Items {
		if answer.Decision != "accept" {
			required[answer.ItemID] = true
		}
	}
	if prior.Reason != "" {
		required["overall"] = true
	}
	for _, r := range review.Resolutions {
		if !known[r.ItemID] {
			return "", ErrInvalid
		}
		delete(required, r.ItemID)
	}
	if len(required) > 0 {
		return "", fmt.Errorf("%w: 未逐项回应上一轮纠偏", ErrInvalid)
	}
	return prior.ID, nil
}
