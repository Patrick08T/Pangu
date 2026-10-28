package scheduler_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/artifact"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

type backupFixture struct {
	Namespace, RunID, ProjectID, VersionID, Workspace string
	Port                                              int
}

func TestHatchetSeedColdBackupFixture(t *testing.T) {
	root := os.Getenv("PANGU_BACKUP_FIXTURE")
	if root == "" {
		t.Skip("仅在显式冷备份演练中保存持久夹具")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("演练目录必须是绝对路径")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("演练目录必须不存在")
	}
	_, client := integrationClient(t)
	b := scheduler.NewBackend(client)
	worker, err := scheduler.NewWorker(client, func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "恢复演练产物"}, os.WriteFile(filepath.Join(input.Workspace, "recovered.txt"), []byte("恢复后仍可查询的真实文件"), 0600)
	}, func(context.Context, workflow.Definition) workflow.Preflight { return workflow.Preflight{Ready: true} })
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })
	awaitWorker(t, b)
	project := backupProject(t, b, root)
	d := workflow.Definition{ProjectID: project.ID, Nodes: []workflow.Node{{ID: "code", Executor: "codex", Handoff: testHandoff(), Instruction: "创建恢复演练文件"}}}
	accepted, err := b.Submit(context.Background(), d, "cold-backup")
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, api.New(b, integrationAccess{}), accepted.RunID)
	object, err := catalog.Create(context.Background(), b.Namespace(), catalog.NewObject{Kind: "template", Name: "恢复模板", Spec: catalog.Spec{Agent: &d.Nodes[0]}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(context.Background(), b.Namespace(), object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	fixture := backupFixture{Namespace: b.Namespace(), RunID: accepted.RunID, ProjectID: project.ID, VersionID: version.ID, Workspace: os.Getenv("PANGU_WORKSPACE_ROOT")}
	if err := conn.QueryRow(context.Background(), "SELECT inet_server_port()").Scan(&fixture.Port); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.Workspace, "fixture.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func backupProject(t *testing.T, b scheduler.Backend, root string) *workspace.Project {
	t.Helper()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("冷备份演练基准"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "--initial-branch=main"}, {"add", "."}, {"-c", "user.name=Pangu", "-c", "user.email=pangu@localhost", "commit", "-m", "演练"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Git: %s %v", output, err)
		}
	}
	t.Setenv("PANGU_WORKSPACE_ROOT", filepath.Join(root, "workspaces"))
	t.Setenv("PANGU_WORKER_ID", "backup-worker")
	p, err := b.RegisterProject(context.Background(), workspace.Project{Name: "恢复演练", Source: source, Branch: "main", WorkerID: "backup-worker"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHatchetVerifyColdBackupCopy(t *testing.T) {
	root := os.Getenv("PANGU_BACKUP_RESTORED")
	if root == "" {
		t.Skip("需要独立恢复副本")
	}
	data, err := os.ReadFile(filepath.Join(root, "workspaces", "fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f backupFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HATCHET_CLIENT_NAMESPACE", strings.TrimSuffix(f.Namespace, "_"+scheduler.WorkflowName))
	t.Setenv("PANGU_WORKSPACE_ROOT", filepath.Join(root, "workspaces"))
	t.Setenv("PANGU_WORKER_ID", "backup-worker")
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var port int
	if err := conn.QueryRow(ctx, "SELECT inet_server_port()").Scan(&port); err != nil || port != backupPort(t, root) || port == backupPort(t, filepath.Join("..", "..")) {
		t.Fatal("拒绝在原数据库进行恢复改写")
	}
	if _, err := conn.Exec(ctx, "UPDATE pangu.workspace_runs SET directory=$2 WHERE run_id=$1 AND directory=$3", f.RunID, filepath.Join(root, "workspaces", f.RunID), filepath.Join(f.Workspace, f.RunID)); err != nil {
		t.Fatal(err)
	}
	client, err := hatchet.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)
	b := scheduler.NewBackend(client)
	run, err := b.GetRun(ctx, f.RunID)
	if err != nil || run.Status != "succeeded" || run.ProjectID != f.ProjectID || len(run.Attempts) != 1 {
		t.Fatalf("历史运行关联未恢复: %+v %v", run, err)
	}
	version, err := catalog.ReadVersion(ctx, f.Namespace, f.VersionID)
	if err != nil || version.Number != 1 || version.Spec.Agent.Instruction != "创建恢复演练文件" {
		t.Fatalf("配置版本未恢复: %+v %v", version, err)
	}
	files, err := artifact.List(ctx, f.RunID)
	if err != nil || len(files.Files) != 1 || files.Files[0].Path != "recovered.txt" {
		t.Fatalf("保留产物未恢复: %+v %v", files, err)
	}
	t.Logf("恢复副本通过：配置版本、提交关联、Hatchet历史、尝试证据、Git产物；port=%d run=%s", port, f.RunID)
}

func backupPort(t *testing.T, root string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".tools", "hatchet-postgres", "data", "postmaster.pid"))
	if err != nil {
		t.Fatal("原实例和副本都须运行，才能核对恢复目标", err)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 4 {
		t.Fatal("数据库进程状态无效")
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[3]))
	if err != nil || port < 1 || port > 65535 {
		t.Fatal("数据库端口无效", err)
	}
	return port
}
