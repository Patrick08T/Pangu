package identity

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
)

func TestPersonalSessionHasFixedThirtyDayLifetime(t *testing.T) {
	ctx, ns := identityDB(t)
	cfg := Config{AppID: "cli_test", Tenant: "tenant", AdminOpenID: "ou_admin"}
	actor, raw, err := login(ctx, ns, cfg, FeishuUser{OpenID: cfg.AdminOpenID, Tenant: cfg.Tenant})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var issued, expires time.Time
	var owner, stored string
	err = conn.QueryRow(ctx, "SELECT created_at,expires_at,user_id::text,token_hash FROM pangu.sessions WHERE token_hash=$1", digest(raw)).Scan(&issued, &expires, &owner, &stored)
	if err != nil {
		t.Fatal(err)
	}
	if expires.Sub(issued) != 30*24*time.Hour {
		t.Fatalf("个人登录凭证应固定 30 天，实际为 %s", expires.Sub(issued))
	}
	if owner != actor.ID || stored == raw {
		t.Fatal("凭证未绑定个人身份或未按摘要存储")
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.sessions SET created_at=created_at-interval '29 days',expires_at=expires_at-interval '29 days' WHERE token_hash=$1", digest(raw))
	if err != nil {
		t.Fatal(err)
	}
	expected := expires.Add(-29 * 24 * time.Hour)
	if got, err := sessionActor(ctx, ns, cfg, raw); err != nil || got.ID != actor.ID {
		t.Fatal("第 29 天应仍可登录", err)
	}
	if err = conn.QueryRow(ctx, "SELECT expires_at FROM pangu.sessions WHERE token_hash=$1", digest(raw)).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if !expires.Equal(expected) {
		t.Fatal("访问时不应延长有效期")
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.sessions SET expires_at=now()-interval '1 second' WHERE token_hash=$1", digest(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sessionActor(ctx, ns, cfg, raw); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("过期凭证仍可访问", err)
	}
}

func TestLoginCompletionIssuesThirtyDayCookieAndReplacesOldSession(t *testing.T) {
	service, actor, old := httpIdentity(t)
	cfg, err := config()
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/auth/callback", nil)
	r.AddCookie(&http.Cookie{Name: "pangu_session", Value: old})
	w := httptest.NewRecorder()
	service.finishLogin(w, r, cfg, FeishuUser{OpenID: cfg.AdminOpenID, Tenant: cfg.Tenant})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != cfg.PublicURL+"/" {
		t.Fatal("登录未返回站点首页", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("未签发独立个人登录 Cookie")
	}
	cookie := cookies[0]
	if cookie.Name != "pangu_session" || cookie.MaxAge != 30*24*60*60 || cookie.Value == old || !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/api" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("个人登录 Cookie 的有效期、替换或保护属性错误")
	}
	if got, err := sessionActor(r.Context(), service.Namespace, cfg, cookie.Value); err != nil || got.ID != actor.ID {
		t.Fatal("Cookie 与个人会话未绑定", err)
	}
	if _, err = sessionActor(r.Context(), service.Namespace, cfg, old); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("重新登录后原会话未撤销", err)
	}
}
