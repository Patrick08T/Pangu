package identity

import (
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestRolesTokenScopesAndMembershipChangesUseCurrentSource(t *testing.T) {
	ctx, ns := identityDB(t)
	cfg := Config{AppID: "cli_test", Tenant: "tenant", AdminOpenID: "ou_admin"}
	admin, _, err := login(ctx, ns, cfg, FeishuUser{OpenID: "ou_admin", Tenant: "tenant"})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	project, foreign := uuid.NewString(), uuid.NewString()
	for _, id := range []string{project, foreign} {
		if _, err = conn.Exec(ctx, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id) VALUES($1,$2,'test','test','main','test')`, id, ns); err != nil {
			t.Fatal(err)
		}
	}
	user, err := Grant(ctx, admin, project, "ou_member", "operator")
	if err != nil {
		t.Fatal(err)
	}
	actor, session, err := login(ctx, ns, cfg, FeishuUser{OpenID: "ou_member", Tenant: "tenant", Name: "成员"})
	if err != nil {
		t.Fatal(err)
	}
	if err = Authorize(ctx, actor, project, "read", ""); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ project, action, owner string }{{foreign, "read", ""}, {project, "manage_members", ""}, {project, "manage_run", admin.ID}} {
		if err = Authorize(ctx, actor, item.project, item.action, item.owner); err == nil {
			t.Fatalf("越权被允许: %+v", item)
		}
	}
	token, raw, err := IssueToken(ctx, actor, project, "CI", []string{"read", "manage_run"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	principal, err := tokenActor(ctx, ns, cfg, raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = Authorize(ctx, principal, project, "manage_run", user.ID); err != nil {
		t.Fatal(err)
	}
	if err = Authorize(ctx, principal, project, "configure", ""); err == nil {
		t.Fatal("Token 超出 scopes")
	}
	if _, err = Grant(ctx, admin, project, "ou_member", "viewer"); err != nil {
		t.Fatal(err)
	}
	if err = Authorize(ctx, principal, project, "manage_run", user.ID); err == nil {
		t.Fatal("角色降级没有即时生效")
	}
	if err = RevokeToken(ctx, actor, token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = tokenActor(ctx, ns, cfg, raw); err == nil {
		t.Fatal("撤销 Token 仍有效")
	}
	if err = Disable(ctx, admin, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = sessionActor(ctx, ns, cfg, session); err == nil {
		t.Fatal("禁用成员后旧会话仍有效")
	}
}
