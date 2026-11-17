//go:build integration

package evolution

import (
	"context"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
)

func TestFullFlowReservationUsesWorkflowObjectAndRealStepTask(t *testing.T) {
	f := newReservationFixture(t)
	ctx := context.Background()
	flow := catalog.Flow{Nodes: []catalog.Binding{{ID: "answer", AgentVersion: f.request.BaselineVersion, Task: "回答该输入的JSON"}}}
	object, err := catalog.Create(ctx, f.scope.Namespace, catalog.NewObject{Kind: "workflow", Name: "real controlled flow", Spec: catalog.Spec{Flow: &flow}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, f.scope.Namespace, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	f.request.FlowObjectID = object.ID
	f.request.Payload.FlowVersion = version.ID
	f.request.Payload.BindingIDs = []string{"answer"}
	f.policy.Workflows = []string{object.ID}
	f.policy.Revision++
	f.request.PolicyRevision = f.policy.Revision
	f.savePolicy(t)
	f.service.Evidence = &executionEvidence{}
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal("真实workflow类型未被预留", err)
	}
	caseID := ""
	for _, c := range f.suite.Cases {
		if c.Partition != "development" {
			caseID = c.ID
			break
		}
	}
	plan, err := f.service.PrepareCase(ctx, f.scope, CaseKey{IntentID: intent.ID, Generation: 1, Arm: "candidate", CaseID: caseID, Repeat: 1})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Definition.VersionID != version.ID || !strings.Contains(plan.Definition.Nodes[0].Instruction, flow.Nodes[0].Task) {
		t.Fatal("未真实编译完整workflow", plan.Definition)
	}
}
