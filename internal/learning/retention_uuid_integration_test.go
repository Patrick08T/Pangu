//go:build integration

package learning

import (
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
)

func TestRetentionUUIDComparisonRejectsMalformedIdentities(t *testing.T) {
	f := newLearningFixture(t)
	tx := f.transaction(t)
	id := uuid.NewString()
	values := map[string]bool{id: true, strings.ToUpper(id): true, strings.ReplaceAll(id, "-", ""): true, "{" + id + "}": true, "urn:uuid:" + id: true, "invalid": false, "{" + id: false, id + "}": false, "urn:uuid:urn:uuid:" + id: false}
	for value, want := range values {
		var same bool
		if err := tx.QueryRow(f.ctx, `SELECT coalesce(`+retentionUUIDSQL("$1::text")+`=$2::uuid,false)`, value, id).Scan(&same); err != nil {
			t.Fatal(err)
		}
		if same != want {
			t.Fatal("incorrect UUID identity match", value, same, want)
		}
	}
}

func TestRetentionWithdrawalMatchesEquivalentSourceUUID(t *testing.T) {
	f := newLearningFixture(t)
	original, _, checker := f.claim(t)
	source := original.Sources[0]
	source.ID = strings.ToUpper(source.ID)
	alias, err := f.store.Create(f.ctx, f.scope, experience.Version{ID: uuid.NewString(), EntryID: uuid.NewString(),
		Revision: 1, ProjectID: f.scope.ProjectID, Content: original.Content, Sources: []experience.Source{source}, ExpiresAt: original.ExpiresAt})
	if err != nil {
		t.Fatal(err)
	}
	if err = WithdrawSource(f.ctx, f.scope, source.ID); err != nil {
		t.Fatal(err)
	}
	if a, b := f.retentionCounts(t, alias.ID, checker.ID); a != 0 || b != 0 {
		t.Fatal("UUID spelling bypassed physical withdrawal", a, b)
	}
}
