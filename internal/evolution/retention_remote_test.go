//go:build integration

package evolution

import (
	"context"
	"testing"
)

func TestRetentionRemoteUnknownRetainsBodiesAndClaimsUntilConfirmed(t *testing.T) {
	base, adapter := rolloutWithExternal(t)
	if _, err := base.service.Reconcile(context.Background(), base.scope, base.id, 1); err != nil {
		t.Fatal(err)
	}
	f := attachRetentionFixture(t, base, "withdrawn")
	rolloutRemote(t, f.rolloutFixture, adapter, "unknown")
	for _, phase := range []string{"unknown", "compensating"} {
		out, err := f.reconcile(t, retentionAllowed)
		if err != nil || !out.Deferred {
			t.Fatal("remote state erased prematurely", phase, out, err)
		}
		c, v, _, claims := f.retained(t)
		if !c || !v || claims != 3 {
			t.Fatal("remote not confirmed but claims/body removed", phase, c, v, claims)
		}
		entry, calls := rolloutRemote(t, f.rolloutFixture, adapter, "confirmed")
		if phase == "unknown" && (calls != 1 || entry.VersionID != f.candidate) {
			t.Fatal("unknown caused blind remote retry", entry, calls)
		}
		if phase == "compensating" && (calls != 2 || entry.VersionID != f.request.BaselineVersion) {
			t.Fatal("bounded rollback not persisted", entry, calls)
		}
	}
	// 新Service只靠DB恢复；可信边界重建，不依赖进程内保留判断。
	f.service = Service{Entries: rolloutEntries{id: f.id, actor: f.scope.ActorID}}
	out, err := f.reconcile(t, retentionAllowed)
	if err != nil || out.Deferred || out.VersionsCleared != 1 {
		t.Fatal("confirmed rollback not erased", out, err)
	}
	c, v, _, claims := f.retained(t)
	if c || v || claims != 0 {
		t.Fatal("confirmed rollback incomplete", c, v, claims)
	}
}
