package execution

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestRecoveryHandlesReusedCurrentPID(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	item := Attempt{RunID: uuid.NewString(), NodeRunID: uuid.NewString(), NodeID: "reuse", Attempt: 1}
	if err := Begin(ctx, &item); err != nil {
		t.Fatal(err)
	}
	stamp, err := agent.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	_, session, hasSession := strings.Cut(stamp, "/")
	oldStamp := "0000000000000001"
	if hasSession {
		oldStamp += "/" + session
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.node_attempts SET owner_stamp=$2 WHERE node_run_id=$1", item.NodeRunID, oldStamp)
	_ = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if active, err := Active(ctx, item.RunID); err != nil || active != 0 {
		t.Fatalf("当前 PID 复用导致恢复遗漏: %d %v", active, err)
	}
}

func TestExecutionOwnerProcess(t *testing.T) {
	if os.Getenv("PANGU_OWNER_FIXTURE") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
}

func TestDeadWorkerReceiptIsRecoveredWithoutCompletingLiveAttempt(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	item := Attempt{RunID: uuid.NewString(), NodeRunID: uuid.NewString(), NodeID: "writer", Attempt: 1}
	if err := Begin(ctx, &item); err != nil {
		t.Fatal(err)
	}
	if err := Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if active, err := Active(ctx, item.RunID); err != nil || active != 1 {
		t.Fatalf("当前 Worker 被误回收: %d %v", active, err)
	}
	binary, _ := os.Executable()
	process := exec.Command(binary, "-test.run=^TestExecutionOwnerProcess$")
	process.Env = append(os.Environ(), "PANGU_OWNER_FIXTURE=1")
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer process.Process.Kill()
	stamp, err := agent.ProcessIdentity(process.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.node_attempts SET owner_pid=$2,owner_stamp=$3 WHERE node_run_id=$1", item.NodeRunID, process.Process.Pid, stamp)
	_ = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = process.Wait()
	if err := Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if active, err := Active(ctx, item.RunID); err != nil || active != 0 {
		t.Fatalf("崩溃记录未回收: %d %v", active, err)
	}
	if err := Begin(ctx, &item); err != nil || item.Attempt != 2 {
		t.Fatalf("没有记录新的物理尝试: %d %v", item.Attempt, err)
	}
	item.Status = "succeeded"
	if err := Finish(ctx, item); err != nil {
		t.Fatal(err)
	}
}
