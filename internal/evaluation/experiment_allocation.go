package evaluation

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func allocateSuiteCase(ctx context.Context, tx pgx.Tx, namespace, project, pool string, c SuiteCase) error {
	_, err := tx.Exec(ctx, `INSERT INTO pangu.evaluation_case_allocations(namespace,project_id,input_hash,partition,pool_id,family_id,task_id,source_run_id)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, namespace, project, c.InputHash, c.Partition, pool, c.FamilyID, c.TaskID, c.SourceRunID)
	if err != nil {
		return err
	}
	var matches bool
	err = tx.QueryRow(ctx, `SELECT partition=$4 AND (partition<>'holdout' OR pool_id=$5) AND family_id=$6 AND task_id=$7 AND source_run_id=$8
  FROM pangu.evaluation_case_allocations WHERE namespace=$1 AND project_id=$2 AND input_hash=$3`, namespace, project, c.InputHash, c.Partition, pool, c.FamilyID, c.TaskID, c.SourceRunID).Scan(&matches)
	if err != nil {
		return err
	}
	if !matches {
		return ErrConflict
	}
	for _, identity := range [][2]string{{"family", c.FamilyID}, {"task", c.TaskID}, {"source", c.SourceRunID}} {
		if err = allocateCaseIdentity(ctx, tx, namespace, project, pool, c.Partition, identity); err != nil {
			return err
		}
	}
	return nil
}

func allocateCaseIdentity(ctx context.Context, tx pgx.Tx, namespace, project, pool, partition string, identity [2]string) error {
	_, err := tx.Exec(ctx, `INSERT INTO pangu.evaluation_identity_allocations(namespace,project_id,identity_kind,identity_key,partition,pool_id)
  VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, namespace, project, identity[0], identity[1], partition, pool)
	if err != nil {
		return err
	}
	var matches bool
	err = tx.QueryRow(ctx, `SELECT partition=$5 AND (partition<>'holdout' OR pool_id=$6) FROM pangu.evaluation_identity_allocations
  WHERE namespace=$1 AND project_id=$2 AND identity_kind=$3 AND identity_key=$4`, namespace, project, identity[0], identity[1], partition, pool).Scan(&matches)
	if err != nil {
		return err
	}
	if !matches {
		return ErrConflict
	}
	return nil
}
