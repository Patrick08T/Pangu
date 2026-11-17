//go:build integration

package scheduler

import (
	"context"
	"testing"
)

func TestExperienceRetentionQueryErrorsAreNotExpiry(t *testing.T) {
	f := newRetentionFixture(t)
	tx, err := f.conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = experienceRetentionDue(context.Background(), tx, f.ns, f.project, f.request); err == nil {
		t.Fatal("closed transaction was interpreted as expiry")
	}
	if f.erased(t) {
		t.Fatal("query failure erased data")
	}
}

func TestExperienceRetentionSourceMismatchIsNotWithdrawal(t *testing.T) {
	f := newRetentionFixture(t)
	f.exec(t, `UPDATE pangu.submissions SET input_hash='unknown' WHERE run_id=$1`, f.source)
	if err := reconcileExperienceRetention(context.Background(), f.ns); err == nil {
		t.Fatal("source mismatch was interpreted as authorized withdrawal")
	}
	if f.erased(t) {
		t.Fatal("unverified source mismatch erased data")
	}
}
