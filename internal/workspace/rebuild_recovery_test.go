package workspace

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func (f rebuildFixture) staleState(t *testing.T, run string) {
	t.Helper()
	_, err := Execute(f.ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "stale fork output"}, os.WriteFile(filepath.Join(dir, "second.txt"), []byte("stale fork changes"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	f.sql(t, `INSERT INTO pangu.approvals(id,namespace,project_id,request_id,run_id,node_id,input_hash,subject_hash,subject,head_commit,status)
 SELECT $1,s.namespace,s.project_id,s.id,s.run_id,'apply','input','subject','{}',w.head_commit,'approved'
 FROM pangu.submissions s JOIN pangu.workspace_runs w ON w.run_id=s.run_id WHERE s.run_id=$2`, uuid.NewString(), run)
}

func TestEnsureRebuildTraversesRerunOfForkWithoutImportingPatch(t *testing.T) {
	f := newRebuildFixture(t)
	fork := f.fork(t)
	rerun := uuid.NewString()
	f.submission(t, rerun, map[string]any{"source_run_id": fork})
	if err := EnsureRerun(f.ctx, rerun, f.project.ID, f.namespace, fork); err != nil {
		t.Fatal(err)
	}
	stale, err := Get(f.ctx, rerun)
	if err != nil || stale.BaseCommit == f.origin {
		t.Fatal("夹具没有污染基线", err)
	}
	child := uuid.NewString()
	if err = EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, rerun); err != nil {
		t.Fatal(err)
	}
	fresh, err := Get(f.ctx, child)
	if err != nil || fresh.BaseCommit != f.origin {
		t.Fatal("重跑 fork 后丢失原始基线", fresh, err)
	}
}

func TestEnsureRebuildRecoversPreparedRunAndKeepsNewCheckpoint(t *testing.T) {
	f := newRebuildFixture(t)
	fork, child := f.fork(t), uuid.NewString()
	if err := EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, fork); err != nil {
		t.Fatal(err)
	}
	f.sql(t, "UPDATE pangu.workspace_runs SET ready=false WHERE run_id=$1", child)
	if err := EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, fork); err != nil {
		t.Fatal(err)
	}
	_, err := Execute(f.ctx, child, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "fresh result"}, os.WriteFile(filepath.Join(dir, "new-result.txt"), []byte("fresh"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := Get(f.ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	if err = EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, fork); err != nil {
		t.Fatal(err)
	}
	after, err := Get(f.ctx, child)
	if err != nil || after.HeadCommit != before.HeadCommit || after.BaseCommit != f.origin {
		t.Fatal("恢复覆盖新成果", err)
	}
	f.sql(t, "UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1", fork)
	if err = EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, fork); err == nil {
		t.Fatal("恢复绕过来源失效")
	}
}

func TestEnsureRebuildRejectsConflictingOrDirtyDestination(t *testing.T) {
	for _, name := range []string{"polluted-base", "wrong-worker", "wrong-directory", "dirty-disk", "expired", "missing-head"} {
		t.Run(name, func(t *testing.T) {
			f := newRebuildFixture(t)
			fork, child := f.fork(t), uuid.NewString()
			if err := EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, fork); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "polluted-base":
				stale, err := Get(f.ctx, fork)
				if err != nil {
					t.Fatal(err)
				}
				f.sql(t, "UPDATE pangu.workspace_runs SET base_commit=$2 WHERE run_id=$1", child, stale.BaseCommit)
			case "wrong-worker":
				f.sql(t, "UPDATE pangu.workspace_runs SET worker_id='foreign' WHERE run_id=$1", child)
			case "wrong-directory":
				f.sql(t, "UPDATE pangu.workspace_runs SET directory='foreign' WHERE run_id=$1", child)
			case "dirty-disk":
				rebuildWrite(t, filepath.Join(f.directory, "w", child, "base", "unexpected.txt"), "dirty")
			case "expired":
				f.sql(t, "UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1", child)
			case "missing-head":
				f.sql(t, "UPDATE pangu.workspace_runs SET head_commit=repeat('f',40) WHERE run_id=$1", child)
			}
			if err := EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, fork); err == nil {
				t.Fatal("恢复接受了冲突工作区")
			}
		})
	}
}

func TestEnsureRebuildSerializesConcurrentCreation(t *testing.T) {
	f := newRebuildFixture(t)
	fork, child := f.fork(t), uuid.NewString()
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			failures <- EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, fork)
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	fresh, err := Get(f.ctx, child)
	if err != nil || fresh.BaseCommit != f.origin || !fresh.Ready {
		t.Fatal("并发重建没有收敛", fresh, err)
	}
}
