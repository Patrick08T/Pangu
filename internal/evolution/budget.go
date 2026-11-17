package evolution

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

// ReserveBudgetTx仅供可信服务。预留即占用，不因失败或策略修订返还；调用方错误必须回滚。
func ReserveBudgetTx(ctx context.Context, tx pgx.Tx, scope Scope, revision int64, kind, key string, amount experience.Budget) error {
	p, err := policyTx(ctx, tx, scope, revision)
	if err != nil {
		return err
	}
	if key == "" || p.BudgetPeriodSeconds <= 0 || (kind != "learning" && kind != "evaluation") {
		return ErrBudget
	}
	var old experience.Budget
	var oldRevision int64
	err = tx.QueryRow(ctx, `SELECT calls,tokens,policy_revision FROM pangu.evolution_budget_charges WHERE namespace=$1 AND project_id=$2 AND kind=$3 AND reservation_key=$4`, scope.Namespace, scope.ProjectID, kind, key).Scan(&old.Calls, &old.Tokens, &oldRevision)
	if err == nil {
		if old != amount || oldRevision != revision {
			return ErrConflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var used experience.Budget
	err = tx.QueryRow(ctx, `SELECT coalesce(sum(calls),0),coalesce(sum(tokens),0) FROM pangu.evolution_budget_charges WHERE namespace=$1 AND project_id=$2 AND kind=$3 AND created_at>=clock_timestamp()-($4::double precision*interval '1 second')`, scope.Namespace, scope.ProjectID, kind, p.BudgetPeriodSeconds).Scan(&used.Calls, &used.Tokens)
	if err != nil {
		return err
	}
	limit := p.EvaluationBudget
	if kind == "learning" {
		limit = p.LearningBudget
	}
	if _, err = reserveBudget(limit, used, amount); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evolution_budget_charges(namespace,project_id,kind,reservation_key,policy_revision,calls,tokens) VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.Namespace, scope.ProjectID, kind, key, revision, amount.Calls, amount.Tokens)
	return err
}
