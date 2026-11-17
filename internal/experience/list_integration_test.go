//go:build integration

package experience

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestStoreListVersionsPagesSafeMetadataAndPreservesTombstones(t *testing.T) {
	f := newStoreFixture(t)
	first, second := f.create(t), f.create(t)
	if err := f.store.Revoke(f.ctx, f.scope, first.ID, "withdrawn"); err != nil {
		t.Fatal(err)
	}
	ids := []string{first.ID, second.ID}
	slices.Sort(ids)
	page, err := f.store.ListVersions(f.ctx, f.scope, Page{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != ids[0] || page.NextCursor != ids[0] {
		t.Fatalf("first metadata page: %+v %v", page, err)
	}
	next, err := f.store.ListVersions(f.ctx, f.scope, Page{Limit: 1, AfterID: page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].ID != ids[1] || next.NextCursor != "" {
		t.Fatalf("next metadata page: %+v %v", next, err)
	}
	for _, item := range append(page.Items, next.Items...) {
		if item.ID == first.ID && item.Validity != "revoked" {
			t.Fatalf("tombstone omitted: %+v", item)
		}
		if item.CreatedAt.IsZero() {
			t.Fatal("missing persisted creation time")
		}
	}
	data, _ := json.Marshal(page)
	for _, private := range []string{"body_hash", "sources", "content", "practice"} {
		if strings.Contains(string(data), private) {
			t.Fatalf("private metadata field leaked: %s", private)
		}
	}
}

func TestStoreListVersionsChecksEverySourceInAuthorizedTransaction(t *testing.T) {
	f := newStoreFixture(t)
	f.create(t)
	var current pgx.Tx
	original := f.store.Authorize
	f.store.Authorize = func(ctx context.Context, tx pgx.Tx, scope Scope) error { current = tx; return original(ctx, tx, scope) }
	checked := false
	f.store.VerifySource = func(_ context.Context, tx pgx.Tx, _ Scope, _ Source) error {
		checked = tx != nil && tx == current
		return ErrForbidden
	}
	page, err := f.store.ListVersions(f.ctx, f.scope, Page{Limit: 10})
	if !checked || !errors.Is(err, ErrForbidden) || len(page.Items) != 0 {
		t.Fatalf("source permission bypass: checked=%v %+v %v", checked, page, err)
	}
}

func TestStoreListsRejectInvalidPaginationAndForeignScope(t *testing.T) {
	f := newStoreFixture(t)
	f.create(t)
	for _, page := range []Page{{Limit: 0}, {Limit: 101}, {Limit: 1, AfterID: "not-uuid"}} {
		if _, err := f.store.ListVersions(f.ctx, f.scope, page); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid page accepted: %+v %v", page, err)
		}
	}
	other := f.scope
	other.Namespace = "foreign"
	if _, err := f.store.ListVersions(f.ctx, other, Page{Limit: 10}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign namespace listed: %v", err)
	}
}
