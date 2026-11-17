package experience

import (
	"context"
	"errors"
	"testing"
)

func TestStoreRejectsMissingAuthorizationBeforeDatabaseAccess(t *testing.T) {
	store := Store{}
	scope := Scope{Namespace: "namespace", ProjectID: "project", ActorID: "actor"}
	_, err := store.Create(context.Background(), scope, sampleVersion("a"))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing auth must fail closed: %v", err)
	}
	_, err = store.FreezeTx(context.Background(), nil, scope, "request", 1)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("transaction freezer missing auth: %v", err)
	}
}
