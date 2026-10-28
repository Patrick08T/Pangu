package identity

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func identityDB(t *testing.T) (context.Context, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return ctx, "identity-" + uuid.NewString()
}

func TestLoginStateIsBrowserBoundSingleUseAndExpires(t *testing.T) {
	ctx, ns := identityDB(t)
	cfg := Config{AppID: "cli_test", AppSecret: "secret", PublicURL: "https://pangu.example", Tenant: "tenant"}
	location, browser, err := beginLogin(ctx, ns, cfg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(location)
	state := parsed.Query().Get("state")
	if _, err = consumeLogin(ctx, ns, cfg, state, "other-browser"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("未绑定浏览器: %v", err)
	}
	verifier, err := consumeLogin(ctx, ns, cfg, state, browser)
	if err != nil || len(verifier) < 43 {
		t.Fatalf("状态不可用: %v", err)
	}
	if _, err = consumeLogin(ctx, ns, cfg, state, browser); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("状态可重放")
	}
	location, browser, err = beginLogin(ctx, ns, cfg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ = url.Parse(location)
	state = parsed.Query().Get("state")
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "UPDATE pangu.login_states SET expires_at=now()-interval '1 second' WHERE state_hash=$1", digest(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = consumeLogin(ctx, ns, cfg, state, browser); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("过期状态仍有效")
	}
}

func TestLoginDoesNotPromoteFirstUserAndSessionRevokesImmediately(t *testing.T) {
	ctx, ns := identityDB(t)
	cfg := Config{AppID: "cli_test", Tenant: "tenant", AdminOpenID: "ou_admin"}
	if _, _, err := login(ctx, ns, cfg, FeishuUser{OpenID: "ou_unknown", Tenant: "tenant", Name: "陌生人"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("首登自动授权: %v", err)
	}
	user, session, err := login(ctx, ns, cfg, FeishuUser{OpenID: "ou_admin", Tenant: "tenant", Name: "管理员"})
	if err != nil {
		t.Fatal(err)
	}
	if !user.Admin || user.ID == "" || session == "" {
		t.Fatal("明确管理员未初始化")
	}
	loaded, err := sessionActor(ctx, ns, cfg, session)
	if err != nil || loaded.ID != user.ID || loaded.CSRF == "" {
		t.Fatalf("会话无效: %+v %v", loaded, err)
	}
	if err := logout(ctx, ns, session); err != nil {
		t.Fatal(err)
	}
	if _, err := sessionActor(ctx, ns, cfg, session); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("退出后旧会话仍有效")
	}
	_, session, err = login(ctx, ns, cfg, FeishuUser{OpenID: "ou_admin", Tenant: "tenant", Name: "管理员"})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "UPDATE pangu.users SET disabled=true WHERE id=$1", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessionActor(ctx, ns, cfg, session); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("禁用后会话仍有效")
	}
	if _, _, err := login(ctx, ns, cfg, FeishuUser{OpenID: "ou_admin", Tenant: "tenant"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("配置管理员重新登录绕过禁用")
	}
}
