package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestPerforceLocalProjectRequiresDirectory(t *testing.T) {
	p := Project{Name: "本地 P4", VCS: "perforce", WorkerID: "worker", Perforce: &perforce.Config{Port: "localhost:1666", User: "tester", Client: "daily", Scope: "."}}
	if p.Validate() == nil {
		t.Fatal("Perforce 必须绑定本地目录")
	}
	p.Source = t.TempDir()
	if err := p.Validate(); err != nil {
		t.Fatal("绑定本地目录被拒绝", err)
	}
}

func TestPerforceLocalUsesBoundAltRoot(t *testing.T) {
	c, p, _, _ := localP4Run(t)
	alt := t.TempDir()
	if err := os.CopyFS(alt, os.DirFS(p.Source)); err != nil {
		t.Fatal(err)
	}
	form := fmt.Sprintf("Client: %s\nOwner: tester\nRoot: %s\nAltRoots:\n\t%s\nOptions: allwrite noclobber nocompress unlocked nomodtime normdir\nLineEnd: unix\nView:\n\t//depot/%s/... //%s/...\n", c.Client, p.Source, alt, c.Client, c.Client)
	p4OK(t, c, form, "client", "-i")
	ctx := context.Background()
	s, err := perforce.CaptureLocal(ctx, c, alt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.LocalFiles(ctx); err != nil {
		t.Fatal("未从绑定的 AltRoot 枚举文件", err)
	}
	if err = os.WriteFile(filepath.Join(alt, "Server", "alt.txt"), []byte("替代根目录"), 0600); err != nil {
		t.Fatal(err)
	}
	s.RunID = uuid.NewString()
	s.WorkChange, err = s.PendingChange(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.TrackLocal(ctx, []string{"alt.txt"}); err != nil {
		t.Fatal(err)
	}
	rows := p4OK(t, c, "", "opened", c.Files(c.Client))
	if len(rows) != 1 || rows[0]["action"] != "add" {
		t.Fatalf("操作没有使用绑定的 AltRoot: %v", rows)
	}
	if _, err = os.Stat(filepath.Join(p.Source, "Server", "alt.txt")); !os.IsNotExist(err) {
		t.Fatal("写入了其他 Root")
	}
}

func localP4Run(t *testing.T) (perforce.Config, *Project, string, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("PANGU_WORKER_ID", "local-"+uuid.NewString())
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c, daily := p4Server(t)
	ns := "local-" + uuid.NewString()
	p, err := Register(ctx, ns, Project{Name: "本地测试", VCS: "perforce", Source: daily, WorkerID: WorkerID(), Perforce: &c})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if err = Ensure(ctx, run, p.ID, ns); err != nil {
		t.Fatal(err)
	}
	return c, p, run, filepath.Join(daily, "Server")
}

func TestPerforceLocalReadOnlyViolationKeepsTrackedChanges(t *testing.T) {
	c, p, run, path := localP4Run(t)
	ctx := context.Background()
	node := uuid.NewString()
	_, err := Execute(ctx, run, node, 1, true, func(dir string) (workflow.NodeOutput, error) {
		revision, err := RevisionFor(ctx, workflow.NodeInput{RunID: run, ProjectID: p.ID, NodeRunID: node, Workspace: dir})
		if err != nil || !revision.LocalPerforce || revision.HeadCommit == "" {
			t.Fatalf("本地目录缺少有效修订信息: %+v %v", revision, err)
		}
		return workflow.NodeOutput{}, os.WriteFile(filepath.Join(dir, "readonly.txt"), []byte("违规修改"), 0600)
	})
	if err == nil || !strings.Contains(err.Error(), "只读节点") {
		t.Fatalf("只读修改未被拒绝: %v", err)
	}
	if _, err = os.Stat(filepath.Join(path, "readonly.txt")); err != nil {
		t.Fatal("丢失现场", err)
	}
	rows := p4OK(t, c, "", "opened", c.Files(c.Client))
	if len(rows) != 1 || rows[0]["change"] == "default" {
		t.Fatalf("违规修改未保留到待审查清单: %v", rows)
	}
}

func TestPerforceLocalRecoversStoppedAttempt(t *testing.T) {
	c, _, run, _ := localP4Run(t)
	point := crashPoint{Run: run, Node: uuid.NewString(), Stage: "after_write", Marker: filepath.Join(t.TempDir(), "ready")}
	killAtBoundary(t, point)
	_, err := Execute(context.Background(), run, point.Node, 2, false, func(dir string) (workflow.NodeOutput, error) {
		body, err := os.ReadFile(filepath.Join(dir, "candidate.txt"))
		if err != nil || string(body) != "待发布文件" {
			t.Fatal("恢复丢失未完成修改", err)
		}
		rows := p4OK(t, c, "", "opened", c.Files(c.Client))
		if len(rows) != 1 || rows[0]["change"] == "default" {
			t.Fatalf("恢复前没有补齐原尝试的 P4 跟踪: %v", rows)
		}
		return workflow.NodeOutput{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPerforceLocalCanReviewStoppedAttemptWithoutRerunningModel(t *testing.T) {
	_, _, run, _ := localP4Run(t)
	point := crashPoint{Run: run, Node: uuid.NewString(), Stage: "after_write", Marker: filepath.Join(t.TempDir(), "ready")}
	killAtBoundary(t, point)
	ctx := context.Background()
	if _, err := QueuePerforceDelivery(ctx, run); err != nil {
		t.Fatal(err)
	}
	out, err := DeliverPerforce(ctx, run)
	if err != nil || out.Status != "shelved" {
		t.Fatalf("已停止的中断现场未能补齐并交付: %+v %v", out, err)
	}
}

func TestPerforceLocalBindingRejectsStaleAndUnfinishedSubmissions(t *testing.T) {
	_, p, _, _ := localP4Run(t)
	ctx := context.Background()
	ns := projectNamespace(t, p.ID)
	if _, err := BindPerforceDirectory(ctx, ns, p.ID, "stale", p.Source); err == nil {
		t.Fatal("覆盖了其他人的目录更新")
	}
	if _, err := BindPerforceDirectory(ctx, ns, p.ID, p.Source, p.Source); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot) VALUES($1::uuid,$2,$3,'run',$1::text,$1::text,'{}')`, uuid.NewString(), ns, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = BindPerforceDirectory(ctx, ns, p.ID, p.Source, p.Source); err == nil {
		t.Fatal("待派发任务期间允许改变目录绑定")
	}
}

func TestPerforceLocalKeepsOriginalChangeWhenDeletingOpenedFile(t *testing.T) {
	c, _, run, _ := localP4Run(t)
	p4OK(t, c, "", "edit", "//"+c.Client+"/Server/delete.txt")
	_, err := Execute(context.Background(), run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{}, os.Remove(filepath.Join(dir, "delete.txt"))
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := p4OK(t, c, "", "opened", c.Files(c.Client))
	if len(rows) != 1 || rows[0]["change"] != "default" || rows[0]["action"] != "delete" {
		t.Fatalf("删除操作或原清单归属不正确: %v", rows)
	}
}

func TestPerforceLocalDeliveryRejectsLaterEditsAndUnrelatedFiles(t *testing.T) {
	for _, later := range []bool{true, false} {
		t.Run(strconv.FormatBool(later), func(t *testing.T) {
			c, _, run, path := localP4Run(t)
			ctx := context.Background()
			_, err := Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
				return workflow.NodeOutput{}, os.WriteFile(filepath.Join(dir, "edit.txt"), []byte("本轮"), 0600)
			})
			if err != nil {
				t.Fatal(err)
			}
			if later {
				if err = os.WriteFile(filepath.Join(path, "edit.txt"), []byte("运行后手工修改"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				delivery, err := PerforceDelivery(ctx, run)
				if err != nil {
					t.Fatal(err)
				}
				p4OK(t, c, "", "edit", "-c", strconv.FormatInt(delivery.Changelist, 10), "//"+c.Client+"/Server/delete.txt")
			}
			delivery, err := DeliverPerforce(ctx, run)
			if err != nil || delivery.Status != "failed" {
				t.Fatalf("不应 shelve 已变更或混入无关文件的清单: %+v %v", delivery, err)
			}
		})
	}
}

func TestPerforceLocalExecutionPreservesDirtyFilesAndChangeOwnership(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c, daily := p4Server(t)
	path := filepath.Join(daily, "Server")
	p4OK(t, c, "", "edit", "//"+c.Client+"/Server/edit.txt")
	if err := os.WriteFile(filepath.Join(path, "edit.txt"), []byte("用户未提交\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ns := "p4-local-" + uuid.NewString()
	p, err := Register(ctx, ns, Project{Name: "本地执行", VCS: "perforce", Source: daily, WorkerID: WorkerID(), Perforce: &c})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if err = Ensure(ctx, run, p.ID, ns); err != nil {
		t.Fatal(err)
	}
	_, err = Execute(ctx, run, uuid.NewString(), 1, false, func(actual string) (workflow.NodeOutput, error) {
		if !samePath(actual, path) {
			t.Fatalf("未直接使用绑定目录: %s", actual)
		}
		body, _ := os.ReadFile(filepath.Join(actual, "edit.txt"))
		if string(body) != "用户未提交\n" {
			t.Fatal("用户修改未被保留")
		}
		if err := os.WriteFile(filepath.Join(actual, "edit.txt"), []byte("用户未提交\n本轮修改\n"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		if err := os.WriteFile(filepath.Join(actual, "new.txt"), []byte("本轮新增\n"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{}, errors.New("模拟模型失败")
	})
	if err == nil {
		t.Fatal("失败被当成成功")
	}
	if _, err = os.Stat(filepath.Join(path, "new.txt")); err != nil {
		t.Fatal("失败时回退了现场", err)
	}
	rows := p4OK(t, c, "", "opened", c.Files(c.Client))
	changes := map[string]string{}
	for _, row := range rows {
		changes[row["clientFile"]] = row["change"]
	}
	if changes["//"+c.Client+"/Server/edit.txt"] != "default" {
		t.Fatalf("原 changelist 归属变化: %v", changes)
	}
	if cl := changes["//"+c.Client+"/Server/new.txt"]; cl == "" || cl == "default" {
		t.Fatalf("新增文件未进入本轮 changelist: %v", changes)
	}
	if _, err = os.Stat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
		t.Fatal("绑定工作区被写入 Git 元数据")
	}
}
