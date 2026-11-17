//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestExperienceRetentionKeysetPassesBusyFirstPage(t *testing.T) {
	f := newRetentionFixture(t)
	f.exec(t, `UPDATE pangu.evolution_case_runtime SET stop_confirmed=false WHERE request_id=$1`, f.request)
	for i := 1; i < experienceRetentionPageSize+2; i++ {
		f.request, f.run = uuid.NewString(), uuid.NewString()
		f.seedRun(t)
		if i < experienceRetentionPageSize {
			f.exec(t, `UPDATE pangu.evolution_case_runtime SET stop_confirmed=false WHERE request_id=$1`, f.request)
		}
	}
	other := newRetentionFixture(t)
	if err := reconcileExperienceRetention(context.Background(), f.ns); err != nil {
		t.Fatal(err)
	}
	var erased, retained int
	err := f.conn.QueryRow(context.Background(), `SELECT count(*) FILTER(WHERE erased_at IS NOT NULL),count(*) FILTER(WHERE erased_at IS NULL) FROM pangu.evolution_case_plans WHERE namespace=$1`, f.ns).Scan(&erased, &retained)
	if err != nil {
		t.Fatal(err)
	}
	if erased != 2 || retained != experienceRetentionPageSize {
		t.Fatalf("erased=%d retained=%d", erased, retained)
	}
	if other.erased(t) {
		t.Fatal("another namespace was purged")
	}
	if err := reconcileExperienceRetention(context.Background(), f.ns); err != nil {
		t.Fatal(err)
	}
	if err := RunEvolutionCase(context.Background(), f.ns, f.request, f.run); err == nil {
		t.Fatal("erased plan was replayed")
	}
}

func TestExperienceRetentionRejectsForgedProjectBinding(t *testing.T) {
	f := newRetentionFixture(t)
	other := newRetentionFixture(t)
	f.exec(t, `UPDATE pangu.submissions SET project_id=$2 WHERE id=$1`, f.request, other.project)
	if err := reconcileExperienceRetention(context.Background(), f.ns); err != nil {
		t.Fatal(err)
	}
	if f.erased(t) {
		t.Fatal("cross-project submission binding was accepted")
	}
}
