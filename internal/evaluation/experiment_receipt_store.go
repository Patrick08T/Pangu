package evaluation

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func RecordExperimentReceipt(ctx context.Context, namespace string, receipt ExperimentReceipt) error {
	_, err := experimentTransaction(ctx, func(tx pgx.Tx) (bool, error) {
		err := RecordExperimentReceiptTx(ctx, tx, namespace, receipt)
		return err == nil, err
	})
	return err
}

// RecordExperimentReceiptTx shares the policy/cancellation fence held by callers.
func RecordExperimentReceiptTx(ctx context.Context, tx pgx.Tx, namespace string, receipt ExperimentReceipt) error {
	s, e, err := readActiveExperiment(ctx, tx, namespace, receipt.ExperimentID)
	if err != nil {
		return err
	}
	if err = validateReceipt(s, e, receipt); err != nil {
		return err
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	hash := contentHash(receipt)
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evaluation_experiment_receipts(experiment_id,generation,arm,case_id,repeat,attempt,receipt,receipt_hash)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, receipt.ExperimentID, receipt.Generation, receipt.Arm, receipt.CaseID, receipt.Repeat, receipt.Attempt, data, hash)
	if err != nil {
		return err
	}
	var saved string
	err = tx.QueryRow(ctx, `SELECT receipt_hash FROM pangu.evaluation_experiment_receipts
   WHERE experiment_id=$1 AND generation=$2 AND arm=$3 AND case_id=$4 AND repeat=$5`, receipt.ExperimentID, receipt.Generation, receipt.Arm, receipt.CaseID, receipt.Repeat).Scan(&saved)
	if err != nil {
		return err
	}
	if saved != hash {
		return ErrConflict
	}
	return nil
}

func storedReceipts(ctx context.Context, tx pgx.Tx, e *FrozenExperiment) ([]ExperimentReceipt, error) {
	rows, err := tx.Query(ctx, `SELECT receipt,receipt_hash FROM pangu.evaluation_experiment_receipts WHERE experiment_id=$1 AND generation=$2 ORDER BY arm,case_id,repeat`, e.ID, e.Generation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	receipts := []ExperimentReceipt{}
	for rows.Next() {
		var data []byte
		var hash string
		var r ExperimentReceipt
		if err = rows.Scan(&data, &hash); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &r); err != nil {
			return nil, err
		}
		if contentHash(r) != hash {
			return nil, ErrConflict
		}
		receipts = append(receipts, r)
	}
	return receipts, rows.Err()
}
