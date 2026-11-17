package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestPerforceShelvesExactChangesOnceAndRetainsReview(t *testing.T) {
	t.Run("classic", func(t *testing.T) { testPerforceDelivery(t, false) })
	t.Run("stream", func(t *testing.T) { testPerforceDelivery(t, true) })
}

func TestPerforceQueueRejectsBusyWorkspaceWithoutDelivery(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	id := uuid.NewString()
	conn, err := lockRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	out, err := QueuePerforceDelivery(ctx, id)
	if out != nil || !errors.Is(err, ErrLeased) {
		t.Fatalf("繁忙且没有交付记录的工作区应拒绝入队: %+v %v", out, err)
	}
}

func testPerforceDelivery(t *testing.T, stream bool) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	c, daily := p4Server(t)
	if stream {
		c = p4Stream(t, c, daily)
	}
	filename := "added space.bin"
	if rows := p4OK(t, c, "", "info"); len(rows) == 1 && rows[0]["unicode"] == "enabled" {
		filename = "中文 空格.bin"
	}
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	namespace := "p4-delivery-" + uuid.NewString()
	project, err := Register(ctx, namespace, Project{VCS: "perforce", Name: "交付测试", Source: daily, WorkerID: WorkerID(), Perforce: &c})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if err = Ensure(ctx, run, project.ID, namespace); err != nil {
		t.Fatal(err)
	}
	_, err = Execute(ctx, run, uuid.NewString(), 1, false, func(dir string) (workflow.NodeOutput, error) {
		if err := os.WriteFile(filepath.Join(dir, "edit.txt"), []byte("new\n"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		if err := os.WriteFile(filepath.Join(dir, filename), []byte{0, 255, 10, 20}, 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		return workflow.NodeOutput{}, os.Remove(filepath.Join(dir, "delete.txt"))
	})
	if err != nil {
		t.Fatal(err)
	}
	assertP4Queue(t, run)
	first, err := DeliverPerforce(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "shelved" || first.Changelist <= 0 {
		t.Fatalf("无 shelf: %+v", first)
	}
	assertP4Review(t, c, daily, run, first)
}

func assertP4Queue(t *testing.T, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	first, err := QueuePerforceDelivery(ctx, id)
	if err != nil || first.Status != "preparing" || !first.Dispatch {
		t.Fatalf("交付没有入队 %+v %v", first, err)
	}
	conn, err := lockRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	second, err := QueuePerforceDelivery(ctx, id)
	if err != nil || second.Dispatch || second.Checkpoint != first.Checkpoint {
		t.Fatalf("运行交付时重复请求被阻塞或重新派发 %+v %v", second, err)
	}
}

func assertP4Review(t *testing.T, c perforce.Config, daily, run string, first *P4Delivery) {
	t.Helper()
	ctx := context.Background()
	loseP4Receipt(t, run)
	second, err := DeliverPerforce(ctx, run)
	if err != nil || first.Changelist != second.Changelist {
		t.Fatalf("重复交付不幂等: %+v %v", second, err)
	}
	described := p4OK(t, c, "", "describe", "-S", "-s", strconv.FormatInt(first.Changelist, 10))
	if len(described) != 1 || described[0]["depotFile3"] != "" || described[0]["depotFile2"] == "" {
		t.Fatalf("shelf 不是精确三项差异: %+v", described)
	}
	checkP4Binary(t, c, described[0], first.Changelist)
	data, _ := os.ReadFile(filepath.Join(daily, "Server", "edit.txt"))
	if string(data) != "new\n" {
		t.Fatal("本地工作区未保留运行修改")
	}
	if err = Expire(ctx, run, time.Now().Add(-31*24*time.Hour)); err == nil || !strings.Contains(err.Error(), "Perforce 交付需要保留") {
		t.Fatal("待审核 shelf 被自动清理")
	}
	info, err := PerforceDelivery(ctx, run)
	if err != nil || info.Changelist != first.Changelist {
		t.Fatal("交付记录丢失", err)
	}
}

func loseP4Receipt(t *testing.T, id string) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.perforce_deliveries SET status='failed',changelist=NULL WHERE run_id=$1", id); err != nil {
		t.Fatal(err)
	}
}

func checkP4Binary(t *testing.T, c perforce.Config, record perforce.Record, change int64) {
	t.Helper()
	for i := 0; i < 3; i++ {
		key := strconv.Itoa(i)
		if record["type"+key] != "binary" {
			continue
		}
		file := filepath.Join(t.TempDir(), "shelved.bin")
		p4OK(t, c, "", "print", "-q", "-o", file, record["depotFile"+key]+"@="+strconv.FormatInt(change, 10))
		data, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(data, []byte{0, 255, 10, 20}) {
			t.Fatalf("shelf 二进制不一致 %v %v", data, err)
		}
		return
	}
	t.Fatal("shelf 未保留二进制类型")
}
