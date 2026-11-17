//go:build integration

package learning

import (
	"testing"
	"time"
)

func TestLearningSourceExposureIsPermanentAndServerBound(t *testing.T) {
	f := newLearningFixture(t)
	f.policy(t)
	source := f.report(t)
	if err := GrantSource(f.ctx, f.scope, source, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var task, inputHash string
	var unresolved bool
	tx := f.transaction(t)
	err := tx.QueryRow(f.ctx, `SELECT e.task_id,e.input_hash,e.unresolved FROM pangu.learning_source_exposures e
	 JOIN pangu.submissions s ON s.run_id=e.run_id WHERE e.namespace=$1 AND e.source_id=$2 AND e.task_id=s.task_id`,
		f.scope.Namespace, source.ID).Scan(&task, &inputHash, &unresolved)
	if err != nil || task == "" || inputHash != digest([]byte("business input")) || unresolved {
		t.Fatal(task, inputHash, unresolved, err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = WithdrawSource(f.ctx, f.scope, source.ID); err != nil {
		t.Fatal(err)
	}
	tx = f.transaction(t)
	_, err = tx.Exec(f.ctx, `DELETE FROM pangu.learning_source_exposures WHERE namespace=$1 AND source_id=$2`, f.scope.Namespace, source.ID)
	if err == nil {
		t.Fatal("source exposure tombstone erased")
	}
}
