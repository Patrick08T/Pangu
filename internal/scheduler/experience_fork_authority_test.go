package scheduler

import (
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestExperienceForkRejectsSameProjectArtifactIdentityMismatch(t *testing.T) {
	f := newExperienceForkFixture(t)
	f.execute(t, 0)
	f.execute(t, 1)
	original := f.snapshot.Fork.Reused["first"]
	a, err := f.source.store.ReadArtifact(f.ctx, f.source.scope, original.Lineage.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	a.ID, a.RunID = uuid.NewString(), uuid.NewString()
	forged := original
	forged.RunID = a.ID
	a.Hash, err = workflow.OutputFingerprint(forged)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.source.store.SaveArtifact(f.ctx, f.source.scope, a); err != nil {
		t.Fatal(err)
	}
	forged.Lineage = &workflow.OutputLineage{Version: 1, ArtifactID: a.ID, InputHash: a.InputHash, OutputHash: a.Hash, ExecutionInputHash: a.ExecutionInputHash}
	f.snapshot.Fork.Reused["first"] = forged
	if err = validateForkExperience(f.ctx, f.ns, f.previous, &f.snapshot); err == nil {
		t.Fatal("same-project proof from other run accepted")
	}
	f.snapshot.Fork.Reused["first"] = original
	proof := *original.Lineage
	proof.ExecutionInputHash = strings.Repeat("a", 64)
	forged = original
	forged.Lineage = &proof
	f.snapshot.Fork.Reused["first"] = forged
	if err = validateForkExperience(f.ctx, f.ns, f.previous, &f.snapshot); err == nil {
		t.Fatal("forged execution hash accepted")
	}
}

func TestExperienceForkChecksCurrentActorRatherThanOriginalCreator(t *testing.T) {
	f := newExperienceForkFixture(t)
	f.execute(t, 0)
	f.execute(t, 1)
	actor := identity.Actor{ID: uuid.NewString(), Namespace: f.ns}
	rebuildExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, actor.ID, f.ns)
	rebuildExec(t, `INSERT INTO pangu.memberships(user_id,project_id,role,enabled) VALUES($1,$2,'operator',true)`, actor.ID, f.source.scope.ProjectID)
	ctx := identity.WithActor(f.ctx, actor)
	if err := validateForkExperience(ctx, f.ns, f.previous, &f.snapshot); err != nil {
		t.Fatal(err)
	}
	rebuildExec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1`, actor.ID)
	if err := validateForkExperience(ctx, f.ns, f.previous, &f.snapshot); err == nil {
		t.Fatal("original creator authority laundered into current actor")
	}
}
