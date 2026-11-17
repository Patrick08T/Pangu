package catalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func PrepareExperimentCandidate(ctx context.Context, namespace, project, baseline string, payload CandidatePayload) (*PreparedCandidate, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return prepareStoredCandidate(ctx, conn, namespace, project, baseline, payload)
}

// ReadVersionScopedTx retains the ordinary project boundary on the caller transaction.
func ReadVersionScopedTx(ctx context.Context, tx pgx.Tx, namespace, id string, scope VersionScope) (*Version, error) {
	return readVersionScoped(ctx, tx, namespace, id, scope)
}

func PrepareExperimentCandidateTx(ctx context.Context, tx pgx.Tx, namespace, project, baseline string, payload CandidatePayload) (*PreparedCandidate, error) {
	return prepareStoredCandidate(ctx, tx, namespace, project, baseline, payload)
}
func StoreExperimentCandidateTx(ctx context.Context, tx pgx.Tx, namespace, experiment string, payload CandidatePayload) error {
	e, err := readExperiment(ctx, tx, namespace, experiment)
	if err != nil {
		return err
	}
	p, err := prepareStoredCandidate(ctx, tx, namespace, e.LogicalProjectID, e.BaselineVersion, payload)
	if err != nil {
		return err
	}
	if err = verifyPrepared(*e, p); err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.catalog_experiment_candidates(experiment_id,payload,payload_hash,candidate_hash) VALUES($1,$2,$3,$4) ON CONFLICT(experiment_id) DO NOTHING`, experiment, data, contentHash(payload), p.CandidateHash)
	if err != nil {
		return err
	}
	stored, err := readCandidate(ctx, tx, experiment)
	if err != nil {
		return err
	}
	if contentHash(stored) != contentHash(payload) {
		return ErrConflict
	}
	return nil
}
func CompileExperimentTx(ctx context.Context, tx pgx.Tx, namespace, experiment, arm, input string) (*workflow.Definition, *ExperimentIdentity, error) {
	if arm != "baseline" && arm != "candidate" {
		return nil, nil, invalid("实验臂无效")
	}
	e, p, _, err := loadExperimentCandidate(ctx, tx, namespace, experiment)
	if err != nil {
		return nil, nil, err
	}
	d := p.Baseline
	if arm == "candidate" {
		d = p.Candidate
	}
	d.Input = input
	if err = d.ValidateInput(); err != nil {
		return nil, nil, invalid(err.Error())
	}
	return &d, e, nil
}

func prepareStoredCandidate(ctx context.Context, reader rowReader, namespace, project, baseline string, payload CandidatePayload) (*PreparedCandidate, error) {
	return prepareCandidate(ctx, project, baseline, payload, func(id string) (*Version, error) {
		return readVersionScoped(ctx, reader, namespace, id, VersionScope{ProjectID: project})
	})
}

func readExperiment(ctx context.Context, reader rowReader, namespace, id string) (*ExperimentIdentity, error) {
	return readExperimentLock(ctx, reader, namespace, id, " FOR SHARE")
}

func readExperimentLock(ctx context.Context, reader rowReader, namespace, id, lock string) (*ExperimentIdentity, error) {
	var data []byte
	var hash, project string
	err := reader.QueryRow(ctx, `SELECT spec,spec_hash,project_id::text FROM pangu.evaluation_experiments
 WHERE namespace=$1 AND id=$2 AND status='active' AND generation=(spec->>'generation')::int`+lock, namespace, id).Scan(&data, &hash, &project)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var e ExperimentIdentity
	if err = json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	if e.ID != id || e.LogicalProjectID != project || e.Generation < 1 || e.PolicyRevision < 1 || e.EnvironmentID == "" {
		return nil, ErrConflict
	}
	e.Hash = hash
	return &e, nil
}

// StoreExperimentCandidate 不接受实验身份，仅用数据库中的受信身份核对候选。
func StoreExperimentCandidate(ctx context.Context, namespace, experiment string, payload CandidatePayload) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = StoreExperimentCandidateTx(ctx, tx, namespace, experiment, payload); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func readCandidate(ctx context.Context, reader rowReader, experiment string) (CandidatePayload, error) {
	var data []byte
	var hash string
	err := reader.QueryRow(ctx, `SELECT payload,payload_hash FROM pangu.catalog_experiment_candidates WHERE experiment_id=$1 AND payload IS NOT NULL FOR SHARE`, experiment).Scan(&data, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidatePayload{}, ErrNotFound
	}
	if err != nil {
		return CandidatePayload{}, err
	}
	var payload CandidatePayload
	if err = json.Unmarshal(data, &payload); err != nil {
		return payload, err
	}
	if contentHash(payload) != hash {
		return payload, ErrConflict
	}
	return payload, nil
}

// CompileExperiment 返回服务端核对的身份供 scheduler 再与执行许可核对，不可用于普通提交绕过校验。
func CompileExperiment(ctx context.Context, namespace, experiment, arm, input string) (*workflow.Definition, *ExperimentIdentity, error) {
	if arm != "baseline" && arm != "candidate" {
		return nil, nil, invalid("实验臂无效")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(context.Background())
	d, e, err := CompileExperimentTx(ctx, tx, namespace, experiment, arm, input)
	if err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return d, e, nil
}

func loadExperimentCandidate(ctx context.Context, reader rowReader, namespace, experiment string) (*ExperimentIdentity, *PreparedCandidate, CandidatePayload, error) {
	e, err := readExperiment(ctx, reader, namespace, experiment)
	if err != nil {
		return nil, nil, CandidatePayload{}, err
	}
	payload, err := readCandidate(ctx, reader, experiment)
	if err != nil {
		return nil, nil, payload, err
	}
	p, err := prepareStoredCandidate(ctx, reader, namespace, e.LogicalProjectID, e.BaselineVersion, payload)
	if err != nil {
		return nil, nil, payload, err
	}
	if err = verifyPrepared(*e, p); err != nil {
		return nil, nil, payload, err
	}
	return e, p, payload, nil
}
