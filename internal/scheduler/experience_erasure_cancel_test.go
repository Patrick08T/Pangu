//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func (f erasureFixture) continuation(t *testing.T, foreign bool) experienceErasureBinding {
	t.Helper()
	child := experienceErasureBinding{f.ns, f.project, uuid.NewString(), uuid.NewString()}
	if foreign {
		child.project = uuid.NewString()
		f.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'foreign','unused','main','test','git')`, child.project, f.ns)
	}
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id) VALUES($1::uuid,$2,$3,'submit',$1::text,$1::text,'{}',$4)`, child.request, child.namespace, child.project, child.run)
	object, version, approval := uuid.NewString(), uuid.NewString(), uuid.NewString()
	f.exec(t, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1,$2,'workflow','test','{}')`, object, f.ns)
	f.exec(t, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) VALUES($1,$2,1,1,'{}')`, version, object)
	f.exec(t, `INSERT INTO pangu.approvals(id,namespace,project_id,request_id,run_id,node_id,input_hash,subject_hash,head_commit) VALUES($1,$2,$3,$4,$5,'gate','input','subject','head')`, approval, f.ns, f.project, f.request, f.run)
	f.exec(t, `INSERT INTO pangu.plan_continuations(approval_id,request_id,version_id) VALUES($1,$2,$3)`, approval, child.request, version)
	return child
}

func TestExperienceErasureCancellationRequiresAuthorityAndWholeScopedClosure(t *testing.T) {
	for _, mode := range []string{"valid", "foreign-child", "generic", "pending-child"} {
		t.Run(mode, func(t *testing.T) {
			cause := "ttl"
			if mode == "generic" {
				cause = "generic"
			}
			f := newErasureFixture(t, cause)
			f.usage(t, f.request, true)
			child := f.continuation(t, mode == "foreign-child")
			if mode == "pending-child" {
				f.exec(t, `UPDATE pangu.submissions SET run_id=NULL WHERE id=$1`, child.request)
			}
			tx, err := f.conn.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			closure, roots, err := experienceErasureCancelTx(context.Background(), tx, experienceErasureBinding{f.ns, f.project, f.request, f.run})
			var count int
			queryErr := tx.QueryRow(context.Background(), `SELECT count(*) FROM pangu.run_cancellations WHERE run_id=ANY($1::uuid[])`, []string{f.run, child.run}).Scan(&count)
			if queryErr != nil {
				t.Fatal(queryErr)
			}
			if mode == "valid" {
				if err != nil || len(closure) != 2 || len(roots) != 1 || count != 2 {
					t.Fatalf("proved closure not durably cancelled: %v %v %d %v", closure, roots, count, err)
				}
			} else if count != 0 || len(closure) > 0 || len(roots) > 0 {
				t.Fatalf("unknown/cross-scope erasure wrote cancellation: %d %v", count, err)
			}
		})
	}
}
