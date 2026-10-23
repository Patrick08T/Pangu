package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type crashPoint struct{ Run, Node, Stage, Marker string }

func TestWorkspaceCrashHelper(t *testing.T) {
	var point crashPoint
	if json.Unmarshal([]byte(os.Getenv("PANGU_CRASH_FIXTURE")), &point) != nil || point.Run == "" {
		t.Skip("仅用于子进程崩溃测试")
	}
	_, err := Execute(context.Background(), point.Run, point.Node, 1, false, func(path string) (workflow.NodeOutput, error) {
		if point.Stage == "before_write" {
			crashMarker(t, point.Marker)
			time.Sleep(time.Minute)
		}
		if err := os.WriteFile(filepath.Join(path, "candidate.txt"), []byte("待发布文件"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
		if point.Stage == "after_write" {
			crashMarker(t, point.Marker)
			time.Sleep(time.Minute)
		}
		return workflow.NodeOutput{Text: "已发布"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	crashMarker(t, point.Marker)
	time.Sleep(time.Minute)
}

func crashMarker(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceFourCrashBoundaries(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("PANGU_WORKER_ID", "crash-matrix")
	t.Setenv("PANGU_WORKSPACE_ROOT", t.TempDir())
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := sourceRepository(t)
	project, err := Register(ctx, "crash-matrix", Project{Name: "四阶段故障", Source: source, Branch: "main", WorkerID: WorkerID()})
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"before_write", "after_write", "before_publish", "after_publish"} {
		t.Run(stage, func(t *testing.T) {
			point := crashPoint{Run: uuid.NewString(), Node: uuid.NewString(), Stage: stage, Marker: filepath.Join(t.TempDir(), "ready")}
			if err := Ensure(ctx, point.Run, project.ID, "crash-matrix"); err != nil {
				t.Fatal(err)
			}
			_, err := Execute(ctx, point.Run, uuid.NewString(), 1, false, func(path string) (workflow.NodeOutput, error) {
				return workflow.NodeOutput{Text: "上游完成"}, os.WriteFile(filepath.Join(path, "preserved.txt"), []byte("上游文件"), 0600)
			})
			if err != nil {
				t.Fatal(err)
			}
			killAtBoundary(t, point)
			assertBoundaryRecovery(t, point)
		})
	}
	if _, err := os.Stat(filepath.Join(source, "candidate.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("崩溃恢复写入了源仓库")
	}
}

func killAtBoundary(t *testing.T, point crashPoint) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if point.Stage == "before_publish" {
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", point.Run); err != nil {
			t.Fatal(err)
		}
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(os.Getenv("PANGU_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("application_name", point.Node)
	u.RawQuery = q.Encode()
	t.Setenv("PANGU_DATABASE_URL", u.String())
	data, _ := json.Marshal(point)
	t.Setenv("PANGU_CRASH_FIXTURE", string(data))
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestWorkspaceCrashHelper$", "-test.timeout=60s")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	awaitBoundary(t, ctx, conn, point)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
}

func awaitBoundary(t *testing.T, ctx context.Context, conn *pgx.Conn, point crashPoint) {
	t.Helper()
	for ctx.Err() == nil {
		if point.Stage == "before_publish" {
			var waiting bool
			err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event='advisory')", point.Node).Scan(&waiting)
			if err != nil {
				t.Fatal(err)
			}
			if waiting {
				return
			}
		} else if _, err := os.Stat(point.Marker); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("未到达预定崩溃边界", point.Stage)
}

func assertBoundaryRecovery(t *testing.T, point crashPoint) {
	t.Helper()
	called := false
	out, err := Execute(context.Background(), point.Run, point.Node, 2, false, func(path string) (workflow.NodeOutput, error) {
		called = true
		if _, err := os.Stat(filepath.Join(path, "candidate.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("未提交的文件污染恢复尝试")
		}
		if data, err := os.ReadFile(filepath.Join(path, "preserved.txt")); err != nil || string(data) != "上游文件" {
			t.Fatal("恢复丢失上游检查点", err)
		}
		return workflow.NodeOutput{Text: "恢复成功"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if point.Stage == "after_publish" {
		if called || out.Text != "已发布" {
			t.Fatal("发布后崩溃导致重复执行或结果丢失")
		}
	} else if !called || out.Text != "恢复成功" {
		t.Fatal("未发布的尝试没有正常恢复")
	}
}
