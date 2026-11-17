//go:build integration

package experience

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestStoreListUsagesPagesOnlySafeExecutionSummary(t *testing.T) {
	f := newStoreFixture(t)
	f.policy(t)
	version := f.serving(t)
	if _, err := f.store.Freeze(f.ctx, f.scope, "list-run", 1); err != nil {
		t.Fatal(err)
	}
	for round := 1; round <= 2; round++ {
		input := SelectionInput{ProjectID: f.scope.ProjectID, RunID: "list-run", NodeID: "node", Round: round, Generation: 1, ApprovalRevision: 1, BoundInputHash: "private-bound", RoleID: "builder", Text: "lock", CanaryScope: "approved"}
		if _, _, err := f.store.Select(f.ctx, f.scope, input); err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.store.ListUsages(f.ctx, f.scope, version.ID, Page{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("first usage page: %+v %v", page, err)
	}
	next, err := f.store.ListUsages(f.ctx, f.scope, version.ID, Page{Limit: 1, AfterID: page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.NextCursor != "" || next.Items[0].Iteration == page.Items[0].Iteration {
		t.Fatalf("next usage page: %+v %v", next, err)
	}
	for _, item := range append(page.Items, next.Items...) {
		if item.RunID != "list-run" || item.NodeID != "node" || item.ApprovalGeneration != 1 || item.ApprovalRevision != 1 || item.CreatedAt.IsZero() {
			t.Fatalf("bad execution summary: %+v", item)
		}
	}
	data, _ := json.Marshal(page)
	for _, private := range []string{"private-bound", "body_hash", "sources", "references", "input", "injection"} {
		if strings.Contains(string(data), private) {
			t.Fatalf("usage leaked %s", private)
		}
	}
	unrelated := f.create(t)
	empty, err := f.store.ListUsages(f.ctx, f.scope, unrelated.ID, Page{Limit: 10})
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("unrelated usages leaked: %+v %v", empty, err)
	}
}

func TestStoreListUsagesRechecksSourcesEvenForEmptyPages(t *testing.T) {
	f := newStoreFixture(t)
	version := f.create(t)
	f.store.VerifySource = func(context.Context, pgx.Tx, Scope, Source) error { return ErrForbidden }
	if _, err := f.store.ListUsages(f.ctx, f.scope, version.ID, Page{Limit: 1}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("usage lookup escaped current source authorization: %v", err)
	}
}

func TestStoreListUsagesRejectsInvalidVersionAndCursor(t *testing.T) {
	f := newStoreFixture(t)
	version := f.create(t)
	for _, page := range []Page{{Limit: 0}, {Limit: 101}, {Limit: 1, AfterID: "not-a-cursor"}} {
		if _, err := f.store.ListUsages(f.ctx, f.scope, version.ID, page); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid usage page accepted: %+v %v", page, err)
		}
	}
	if _, err := f.store.ListUsages(f.ctx, f.scope, "bad-id", Page{Limit: 10}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid version accepted: %v", err)
	}
}
