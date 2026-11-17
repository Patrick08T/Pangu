//go:build integration

package scheduler

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (f erasureFixture) nextRequest(t *testing.T) erasureFixture {
	t.Helper()
	f.request = uuid.NewString()
	f.run = uuid.NewString()
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id) VALUES($1::uuid,$2,$3,'submit',$1::text,$1::text,'{"purpose":"business","workflow":{"input":"private-input"}}',$4)`, f.request, f.ns, f.project, f.run)
	f.usage(t, f.request, true)
	return f
}

func TestExperienceErasureReconcileAdvancesPastEightPermanentlyDeferredConsumers(t *testing.T) {
	b := erasureBackendHTTP(t, "404")
	f := newErasureFixtureNamespace(t, "ttl", b.Namespace())
	items := make([]erasureFixture, 9)
	for i := range items {
		items[i] = f.nextRequest(t)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].request < items[j].request })
	items[8].stopReceipt(t, items[8].run)
	if err := b.ReconcileExperienceErasure(context.Background()); err == nil {
		t.Fatal("unknown first-page stops disappeared")
	}
	var count int
	if err := f.conn.QueryRow(context.Background(), `SELECT count(*) FROM pangu.run_cancellations c JOIN pangu.submissions s ON s.run_id=c.run_id WHERE s.project_id=$1`, f.project).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 8 {
		t.Fatal("pass exceeded eight-consumer bound", count)
	}
	if err := b.ReconcileExperienceErasure(context.Background()); err != nil {
		t.Fatal("second page starved", err)
	}
	var purged bool
	if err := f.conn.QueryRow(context.Background(), `SELECT purged_at IS NOT NULL FROM pangu.submissions WHERE id=$1`, items[8].request).Scan(&purged); err != nil || !purged {
		t.Fatal("ninth proved consumer was never erased", err, purged)
	}
}

func TestExperienceErasureReconcileProjectFailureDoesNotStarveLaterProject(t *testing.T) {
	b := erasureBackendHTTP(t, "404")
	items := []erasureFixture{newErasureFixtureNamespace(t, "ttl", b.Namespace()), newErasureFixtureNamespace(t, "ttl", b.Namespace())}
	sort.Slice(items, func(i, j int) bool { return items[i].project < items[j].project })
	for _, f := range items {
		f.usage(t, f.request, true)
		f.stopReceipt(t, f.run)
	}
	first, last := items[0], items[1]
	key := "experience:" + first.ns + ":" + first.project
	first.exec(t, `SELECT pg_advisory_lock(hashtextextended($1,0))`, key)
	defer first.exec(t, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, key)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := b.ReconcileExperienceErasure(ctx); err == nil {
		t.Fatal("blocked project timeout hidden")
	}
	var purged bool
	if err := last.conn.QueryRow(context.Background(), `SELECT purged_at IS NOT NULL FROM pangu.submissions WHERE id=$1`, last.request).Scan(&purged); err != nil || !purged {
		t.Fatal("failed first project starved later erasure", err, purged)
	}
}
