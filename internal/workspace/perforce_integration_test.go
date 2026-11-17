package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func p4Server(t *testing.T) (perforce.Config, string) {
	t.Helper()
	port := os.Getenv("PANGU_TEST_P4_PORT")
	if port == "" {
		t.Skip("需要独立本地 P4 测试服务，运行 scripts/test-perforce.ps1")
	}
	c := perforce.Config{Port: port, User: "tester", Client: "test-" + uuid.NewString(), Scope: "Server"}
	if len(port) < 10 || port[:10] != "127.0.0.1:" {
		t.Fatal("拒绝在非本机测试服务写入")
	}
	daily := t.TempDir()
	form := fmt.Sprintf("Client: %s\nOwner: tester\nRoot: %s\nOptions: allwrite noclobber nocompress unlocked nomodtime normdir\nLineEnd: unix\nView:\n\t//depot/%s/... //%s/...\n", c.Client, daily, c.Client, c.Client)
	p4OK(t, c, form, "client", "-i")
	for name, content := range map[string]string{"edit.txt": "old\n", "delete.txt": "delete\n", ".gitignore": "ignored.txt\n", "ignored.txt": "tracked\n"} {
		file := filepath.Join(daily, "Server", name)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p4OK(t, c, "", "add", c.Files(c.Client))
	p4OK(t, c, "", "submit", "-d", "baseline")
	return c, daily
}

func p4OK(t *testing.T, c perforce.Config, input string, args ...string) []perforce.Record {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := perforce.Command(ctx, c, c.Client, input, args...)
	if err != nil {
		t.Fatalf("p4 %v: %v", args, err)
	}
	return out
}

func TestPerforceLocalBaselineAndRerun(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	c, daily := p4Server(t)
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	project, err := Register(ctx, "p4-test-"+uuid.NewString(), Project{VCS: "perforce", Name: "P4 集成", Source: daily, WorkerID: WorkerID(), Perforce: &c})
	if err != nil {
		t.Fatal(err)
	}
	// 本地执行以当前工作区（含未提交修改）为起点。
	if err = os.WriteFile(filepath.Join(daily, "Server", "edit.txt"), []byte("日常未提交"), 0600); err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	namespace := projectNamespace(t, project.ID)
	if err = Ensure(ctx, run, project.ID, namespace); err != nil {
		t.Fatal(err)
	}
	record, err := Get(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	data, cut, err := gitOutput(ctx, filepath.Join(record.Directory, "base"), nil, 1024, "show", record.BaseCommit+":edit.txt")
	if err != nil || cut || string(data) != "日常未提交" {
		t.Fatalf("基线未保留日常修改: %q %v", data, err)
	}
	if _, err = git(ctx, filepath.Join(record.Directory, "base"), "cat-file", "-e", record.BaseCommit+":ignored.txt"); err != nil {
		t.Fatal("丢失 P4 已跟踪的忽略文件", err)
	}
	_, err = Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{}, os.WriteFile(filepath.Join(dir, "edit.txt"), []byte("iteration\n"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	next := uuid.NewString()
	if err = EnsureRerun(ctx, next, project.ID, namespace, run); err != nil {
		t.Fatal(err)
	}
	previous, _ := Get(ctx, next)
	data, cut, err = gitOutput(ctx, filepath.Join(previous.Directory, "base"), nil, 1024, "show", previous.BaseCommit+":edit.txt")
	if err != nil || cut || string(data) != "iteration\n" {
		t.Fatal("新运行未基于本地现状建立快照", err)
	}
	data, _ = os.ReadFile(filepath.Join(daily, "Server", "edit.txt"))
	if string(data) != "iteration\n" {
		t.Fatal("工作区没有保留本轮修改")
	}
	assertP4ForkDelivery(t, run, project.ID, namespace)
}

func assertP4ForkDelivery(t *testing.T, source, project, namespace string) {
	t.Helper()
	ctx := context.Background()
	record, err := Get(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	fork := uuid.NewString()
	if err = EnsureFork(ctx, fork, project, namespace, source, record.HeadCommit); err != nil {
		t.Fatal(err)
	}
	// 没有新增修改的返工保留现状，不把原运行 changelist 的文件迁移到新清单。
	out, err := DeliverPerforce(ctx, fork)
	if err != nil || out.Status != "empty" {
		t.Fatalf("返工丢失复用产物: %+v %v", out, err)
	}
}

func projectNamespace(t *testing.T, id string) string {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var value string
	if err = conn.QueryRow(context.Background(), "SELECT namespace FROM pangu.projects WHERE id=$1", id).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func p4Stream(t *testing.T, c perforce.Config, daily string) perforce.Config {
	t.Helper()
	name := "streams" + uuid.NewString()[:8]
	p4OK(t, c, fmt.Sprintf("Depot: %s\nOwner: tester\nType: stream\nMap: %s/...\nStreamDepth: 2\n", name, name), "depot", "-i")
	stream := "//" + name + "/main"
	c.Client += "-s"
	p4OK(t, c, fmt.Sprintf("Stream: %s\nOwner: tester\nName: main\nParent: none\nParentView: inherit\nType: mainline\nPaths:\n\tshare ...\n", stream), "stream", "-i")
	p4OK(t, c, fmt.Sprintf("Client: %s\nOwner: tester\nRoot: %s\nOptions: allwrite noclobber nocompress unlocked nomodtime normdir\nLineEnd: unix\nStream: %s\n", c.Client, daily, stream), "client", "-i")
	p4OK(t, c, "", "add", c.Files(c.Client))
	p4OK(t, c, "", "submit", "-d", "stream baseline")
	return c
}
