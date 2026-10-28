package api

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"testing"
)

func TestFactoryOnlyOwnerOrManagerCanContinueIncludingTransaction(t *testing.T) {
	h, ns, project, _, token := authorizationAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var user string
	if err = conn.QueryRow(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1 RETURNING user_id::text", project).Scan(&user); err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	if _, err = conn.Exec(ctx, "INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1,$2,'test','tenant',$3)", owner, ns, owner); err != nil {
		t.Fatal(err)
	}
	s, err := factory.Create(ctx, ns, project)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.factory_sessions SET created_by=$2 WHERE id=$1", s.ID, owner); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"messages", "apply"} {
		w := callAuthorized(h, "POST", "/factory/sessions/"+s.ID+"/"+path, `{}`, token)
		if w.Code != 403 {
			t.Fatal("其他操作者可写", w.Code, w.Body.String())
		}
	}
	actor := identity.WithActor(ctx, identity.Actor{ID: user, Namespace: ns})
	if _, err = factory.Apply(actor, ns, s.ID, uuid.NewString(), 1); !errors.Is(err, identity.ErrForbidden) {
		t.Fatal("事务内未验证所有权", err)
	}
}
