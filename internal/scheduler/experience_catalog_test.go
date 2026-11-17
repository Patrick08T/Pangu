package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

// A revoked root flow must not survive merely because its individual roles are still valid.
func acceptedAutomaticFlow(t *testing.T) (entryFixture, *submission.Record, string) {
	t.Helper()
	f := newEntryFixture(t)
	object, err := catalog.Create(f.ctx, f.actor.Namespace, catalog.NewObject{Kind: "workflow", Name: "accepted flow", Spec: catalog.Spec{Flow: &catalog.Flow{Nodes: []catalog.Binding{{ID: "answer", AgentVersion: f.version.ID}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f.version, err = catalog.Publish(f.ctx, f.actor.Namespace, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	version := f.automatic(t, f.project, "observing")
	f.adopt(t, version)
	record, err := f.submit("task", "frozen-flow")
	if err != nil {
		t.Fatal(err)
	}
	return f, record, version
}

func TestRootRejectsRevokedAcceptedCatalogBeforeWorkspace(t *testing.T) {
	f, record, version := acceptedAutomaticFlow(t)
	input := runInput{RequestID: record.ID, Snapshot: record.Snapshot}
	run := uuid.NewString()
	if _, err := loadRootExperience(context.Background(), f.actor.Namespace, run, input); err != nil {
		t.Fatal("valid observing entry rejected", err)
	}
	entryExec(t, `UPDATE pangu.catalog_version_state SET state='revoked',revision=revision+1 WHERE version_id=$1`, version)
	if _, err := loadRootExperience(context.Background(), f.actor.Namespace, run, input); err == nil {
		t.Fatal("revoked accepted root reached workspace boundary")
	}
}

func TestAcceptedCatalogRechecksTransitivePrivateAncestor(t *testing.T) {
	f, _, ancestor := acceptedAutomaticFlow(t)
	version := f.automaticFrom(t, f.project, "observing", ancestor)
	f.adopt(t, version)
	record, err := f.submit("task", "derived-flow")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	run := uuid.NewString()
	runtime, err := loadNodeExperience(ctx, f.actor.Namespace, record.ID, run, f.project)
	if err != nil {
		t.Fatal("valid ancestry rejected", err)
	}
	input := record.Snapshot.Workflow.NodeInput(record.Snapshot.Workflow.Nodes[0], nil)
	if err = runtime.Prepare(ctx, &input); err != nil {
		t.Fatal(err)
	}
	entryExec(t, `UPDATE pangu.catalog_version_state SET state='revoked',revision=revision+1 WHERE version_id=$1`, ancestor)
	if _, err = loadRootExperience(ctx, f.actor.Namespace, run, runInput{RequestID: record.ID, Snapshot: record.Snapshot}); err == nil {
		t.Error("derived private root survived revoked ancestor")
	}
	if err = runtime.checkDispatch(ctx, input); err == nil {
		t.Error("loaded worker reused revoked ancestor influence")
	}
}

func TestWorkerRechecksRootCatalogAfterRuntimeLoad(t *testing.T) {
	f, record, version := acceptedAutomaticFlow(t)
	ctx := context.Background()
	runtime, err := loadNodeExperience(ctx, f.actor.Namespace, record.ID, uuid.NewString(), f.project)
	if err != nil {
		t.Fatal(err)
	}
	input := record.Snapshot.Workflow.NodeInput(record.Snapshot.Workflow.Nodes[0], nil)
	if err = runtime.Prepare(ctx, &input); err != nil {
		t.Fatal("valid observing flow rejected", err)
	}
	entryExec(t, `UPDATE pangu.catalog_version_state SET state='revoked',revision=revision+1 WHERE version_id=$1`, version)
	if err = runtime.checkDispatch(ctx, input); err == nil {
		t.Fatal("worker dispatched revoked root with still-valid role")
	}
}
