package web_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/Patrick08T/Pangu/web"
	"github.com/google/uuid"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

// TestBrowserConsole 仅测试构建包含临时登录夹具，不编译进产品；真实飞书验收单独执行。
func TestBrowserConsole(t *testing.T) {
	if os.Getenv("PANGU_BROWSER_TEST") != "1" {
		t.Skip("浏览器验收显式开启")
	}
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Fatal("需要独立测试数据库")
	}
	ctx := context.Background()
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	t.Setenv("HATCHET_CLIENT_NAMESPACE", "browser_"+uuid.NewString())
	for key, value := range map[string]string{"PANGU_FEISHU_APP_ID": "browser-test", "PANGU_FEISHU_APP_SECRET": "test-only", "PANGU_FEISHU_TENANT_KEY": "browser-test", "PANGU_PUBLIC_URL": "http://127.0.0.1:8083", "PANGU_WORKER_ID": "browser-worker", "PANGU_WORKSPACE_ROOT": t.TempDir()} {
		t.Setenv(key, value)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	client, err := hatchet.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)
	backend := scheduler.NewBackend(client)
	worker, err := scheduler.NewWorker(client, func(_ context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		return workflow.NodeOutput{Text: "浏览器验收夹具输出：<script>alert('文本')</script>，未调用真实模型。"}, nil
	}, func(context.Context, workflow.Definition) workflow.Preflight {
		return workflow.Preflight{Ready: true, Checks: []workflow.Check{{Component: "测试执行器", Status: "configured"}}}
	})
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	raw, actor := browserIdentity(t, backend.Namespace())
	project, err := workspace.Register(ctx, backend.Namespace(), workspace.Project{Name: "浏览器验收项目", Source: browserGit(t), Branch: "main", WorkerID: "browser-worker"})
	if err != nil {
		t.Fatal(err)
	}
	browserCatalog(t, actor, project.ID)
	handler := web.Handler(api.New(backend, identity.Service{Namespace: backend.Namespace()}))
	browserServe(t, handler, raw)
}

func browserServe(t *testing.T, handler http.Handler, raw string) {
	t.Helper()
	stopped := make(chan struct{}, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__test/login" {
			http.SetCookie(w, &http.Cookie{Name: "pangu_session", Value: raw, Path: "/api", HttpOnly: true, SameSite: http.SameSiteLaxMode})
			http.Redirect(w, r, "/", 303)
			return
		}
		if r.URL.Path == "/__test/stop" && r.Method == "POST" {
			select {
			case stopped <- struct{}{}:
			default:
			}
			w.WriteHeader(204)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	server.Listener.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:8083")
	if err != nil {
		t.Fatal(err)
	}
	server.Listener = listener
	server.Start()
	defer server.Close()
	t.Log("浏览器验收入口 http://127.0.0.1:8083/__test/login；真实 API/PG/Hatchet，测试身份和执行器；最长 15 分钟")
	select {
	case <-stopped:
	case <-time.After(15 * time.Minute):
		t.Fatal("浏览器验收超时")
	}
}

func browserIdentity(t *testing.T, namespace string) (string, identity.Actor) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	actor := identity.Actor{ID: uuid.NewString(), Namespace: namespace, Admin: true, Name: "浏览器验收管理员"}
	random := sha256.Sum256([]byte(uuid.NewString()))
	raw := base64.RawURLEncoding.EncodeToString(random[:])
	hash := sha256.Sum256([]byte(raw))
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,name,is_admin) VALUES($1,$2,'browser-test','browser-test','ou_browser',$3,true)`, actor.ID, namespace, actor.Name); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '20 minutes')`, hex.EncodeToString(hash[:]), actor.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := database.Open(ctx)
		if err == nil {
			defer conn.Close(ctx)
			conn.Exec(ctx, "DELETE FROM pangu.sessions WHERE user_id=$1", actor.ID)
		}
	})
	return raw, actor
}

func browserGit(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# 浏览器验收项目\n用于隔离的页面验收。\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "README.md"}, {"-c", "user.name=Pangu", "-c", "user.email=test@example.invalid", "commit", "-m", "浏览器夹具"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Git: %v %s", err, data)
		}
	}
	return root
}

func browserCatalog(t *testing.T, actor identity.Actor, project string) {
	t.Helper()
	ctx := identity.WithActor(context.Background(), actor)
	object, err := catalog.Create(ctx, actor.Namespace, catalog.NewObject{Kind: "agent", Name: "浏览器文本安全验收", Spec: catalog.Spec{Agent: &workflow.Node{ID: "review", Executor: "codex", Instruction: "测试执行器，不调用真实模型"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.Publish(ctx, actor.Namespace, object.ID, object.Revision); err != nil {
		t.Fatal(err)
	}
}
