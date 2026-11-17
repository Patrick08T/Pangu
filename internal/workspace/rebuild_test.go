package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestEnsureRebuildForkStartsAtCleanOriginAndPreservesSource(t *testing.T) {
	f := newRebuildFixture(t)
	sourceID := f.fork(t)
	f.staleState(t, sourceID)
	source, err := Get(f.ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.conn.Exec(f.ctx, "INSERT INTO pangu.run_cancellations(run_id,namespace) VALUES($1,$2)", sourceID, f.namespace)
	if err != nil {
		t.Fatal(err)
	}
	rebuildWrite(t, filepath.Join(f.source, "latest.txt"), "new branch")
	rebuildGit(t, f.ctx, f.source, "add", ".")
	rebuildGit(t, f.ctx, f.source, "commit", "-m", "branch advanced")
	rebuildWrite(t, filepath.Join(source.Directory, "base", "untracked.txt"), "old source must stay")
	child := uuid.NewString()
	for range 2 {
		if err = EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, sourceID); err != nil {
			t.Fatal(err)
		}
	}
	fresh, err := Get(f.ctx, child)
	if err != nil || fresh.BaseCommit != f.origin || fresh.HeadCommit != f.origin || !fresh.Ready {
		t.Fatalf("重建继承了受污染的 fork 基线: %+v %v; clean=%s", fresh, err, f.origin)
	}
	for _, name := range []string{"polluted.txt", "second.txt", "latest.txt", "untracked.txt", ".git/objects/info/alternates"} {
		if _, err = os.Stat(filepath.Join(fresh.Directory, "base", name)); !os.IsNotExist(err) {
			t.Fatalf("重建继承 %s: %v", name, err)
		}
	}
	for _, table := range []string{"workspace_checkpoints", "research_reports", "workspace_attempts", "approvals"} {
		var count int
		if err = f.conn.QueryRow(f.ctx, "SELECT count(*) FROM pangu."+table+" WHERE run_id=$1", child).Scan(&count); err != nil || count != 0 {
			t.Fatalf("导入了旧成果 %s: %d %v", table, count, err)
		}
	}
	unchanged, err := Get(f.ctx, sourceID)
	if err != nil || unchanged.HeadCommit != source.HeadCommit || unchanged.BaseCommit != source.BaseCommit {
		t.Fatal("原工作区被改变", err)
	}
	var cancelled bool
	if err = f.conn.QueryRow(f.ctx, "SELECT EXISTS(SELECT 1 FROM pangu.run_cancellations WHERE run_id=$1)", sourceID).Scan(&cancelled); err != nil || !cancelled {
		t.Fatal("原取消意图被撤销", err)
	}
	data, err := os.ReadFile(filepath.Join(source.Directory, "base", "untracked.txt"))
	if err != nil || string(data) != "old source must stay" {
		t.Fatal("原磁盘被改变", err)
	}
	_, err = Execute(f.ctx, child, uuid.NewString(), 1, true, func(dir string) (workflow.NodeOutput, error) {
		data, e := os.ReadFile(filepath.Join(dir, "source.txt"))
		if e != nil || string(data) != "clean origin" {
			t.Fatal("新节点不是干净基线", e)
		}
		return workflow.NodeOutput{Text: "new isolated result"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRebuildOriginalUsesBaseNotHead(t *testing.T) {
	f := newRebuildFixture(t)
	_ = f.fork(t)
	child := uuid.NewString()
	if err := EnsureRebuild(f.ctx, child, f.project.ID, f.namespace, f.run); err != nil {
		t.Fatal(err)
	}
	fresh, err := Get(f.ctx, child)
	if err != nil || fresh.BaseCommit != f.origin || fresh.HeadCommit != f.origin {
		t.Fatal("使用了来源结果", fresh, err)
	}
}

func TestEnsureRebuildRejectsWrongIdentityAndUnavailableSource(t *testing.T) {
	for _, name := range []string{"same-run", "empty-source", "namespace", "project", "worker", "source-worker", "source-directory", "source-expired", "source-unready", "missing-submission", "submission-namespace", "missing-origin", "invalid-origin", "nonancestor-origin", "perforce"} {
		t.Run(name, func(t *testing.T) {
			f := newRebuildFixture(t)
			source := f.fork(t)
			run, project, namespace := uuid.NewString(), f.project.ID, f.namespace
			switch name {
			case "same-run":
				run = source
			case "empty-source":
				source = ""
			case "namespace":
				namespace = "foreign"
			case "project":
				project = uuid.NewString()
			case "worker":
				t.Setenv("PANGU_WORKER_ID", "foreign")
			case "source-worker":
				f.sql(t, "UPDATE pangu.workspace_runs SET worker_id='foreign' WHERE run_id=$1", source)
			case "source-directory":
				f.sql(t, "UPDATE pangu.workspace_runs SET directory='foreign' WHERE run_id=$1", source)
			case "source-expired":
				f.sql(t, "UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1", source)
			case "source-unready":
				f.sql(t, "UPDATE pangu.workspace_runs SET ready=false WHERE run_id=$1", source)
			case "missing-submission":
				f.sql(t, "DELETE FROM pangu.submissions WHERE run_id=$1", source)
			case "submission-namespace":
				f.sql(t, "UPDATE pangu.submissions SET namespace='foreign' WHERE run_id=$1", source)
			case "missing-origin":
				f.sql(t, "UPDATE pangu.submissions SET snapshot=snapshot #- '{fork,origin_commit}' WHERE run_id=$1", source)
			case "invalid-origin":
				f.sql(t, "UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{fork,origin_commit}','\"not-a-commit\"') WHERE run_id=$1", source)
			case "nonancestor-origin":
				record, e := Get(f.ctx, source)
				if e != nil {
					t.Fatal(e)
				}
				base := filepath.Join(record.Directory, "base")
				tree := rebuildGit(t, f.ctx, base, "rev-parse", "HEAD^{tree}")
				other := rebuildGit(t, f.ctx, base, "commit-tree", tree, "-m", "unrelated root")
				f.sql(t, "UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{fork,origin_commit}',to_jsonb($2::text)) WHERE run_id=$1", source, other)
			case "perforce":
				f.sql(t, "UPDATE pangu.projects SET vcs='perforce',perforce='{}'::jsonb WHERE id=$1", project)
			}
			if err := EnsureRebuild(f.ctx, run, project, namespace, source); err == nil {
				t.Fatal("未拒绝不安全重建")
			}
			if run != source {
				if _, err := Get(f.ctx, run); err == nil {
					t.Fatal("拒绝前已创建目标记录")
				}
				if _, err := os.Stat(filepath.Join(f.directory, "w", run)); !os.IsNotExist(err) {
					t.Fatal("拒绝前已写入目标目录", err)
				}
			}
			if name == "submission-namespace" {
				f.sql(t, "UPDATE pangu.submissions SET namespace=$2 WHERE run_id=$1", source, f.namespace)
			}
		})
	}
}

func (f rebuildFixture) sql(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.conn.Exec(f.ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}
