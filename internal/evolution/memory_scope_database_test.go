//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestMemoryScopeChecksRoleVersionAndUnpublishedCandidate(t *testing.T) {
	for _, kind := range []string{"role", "version", "changed_candidate", "unversioned_role"} {
		t.Run(kind, func(t *testing.T) {
			f := newReservationFixture(t)
			a := experience.Applicability{Roles: []string{f.request.AgentObjectID}}
			if kind == "role" {
				a.Roles = []string{uuid.NewString()}
			}
			if kind == "version" {
				a.RoleVersions = []string{uuid.NewString()}
			}
			if kind == "changed_candidate" {
				a.RoleVersions = []string{f.request.BaselineVersion}
			}
			m := benefitScopedMemory(t, &f, a)
			f.request.MemoryVersionIDs = []string{m.ID}
			_, err := f.service.ReserveCandidate(context.Background(), f.scope, f.request)
			if kind == "unversioned_role" {
				if err != nil {
					t.Fatal("RoleVersions缺省被错误收窄", err)
				}
				return
			}
			if !errors.Is(err, ErrForbidden) {
				t.Fatal("不适用memory被实际role实验接受", err)
			}
		})
	}
}

func TestMemoryScopeChecksEveryActualFlowRoleWithoutFiltering(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "reject_unlisted_node", true: "all_nodes_authorized"}[allowed], func(t *testing.T) {
			f := newReservationFixture(t)
			f.service.Evidence = &executionEvidence{}
			other, version := attachMemoryScopeFlow(t, &f)
			a := experience.Applicability{Roles: []string{f.request.AgentObjectID}, RoleVersions: []string{f.request.BaselineVersion, version}}
			if allowed {
				a.Roles = append(a.Roles, other)
			}
			m := benefitScopedMemory(t, &f, a)
			f.request.Kind = "memory_benefit"
			f.request.Payload.Agent = workflow.Node{}
			f.request.BenefitMemoryVersionID = m.ID
			f.request.MemoryVersionIDs = []string{m.ID}
			intent, err := f.service.ReserveCandidate(context.Background(), f.scope, f.request)
			if !allowed {
				if !errors.Is(err, ErrForbidden) {
					t.Fatal("部分flow节点越过memory范围", err)
				}
				return
			}
			if err != nil {
				t.Fatal("全体实际role/version授权仍被拒绝", err)
			}
			for _, c := range f.suite.Cases {
				if c.Partition != "development" {
					plan, e := f.service.PrepareCase(context.Background(), f.scope, CaseKey{IntentID: intent.ID, Generation: 1, Arm: "candidate", CaseID: c.ID, Repeat: 1})
					if e != nil || len(plan.Definition.Nodes) != 2 || len(plan.Memories) != 1 {
						t.Fatal("current实际scope校验失败或静默过滤memory", e)
					}
					break
				}
			}
		})
	}
}

func attachMemoryScopeFlow(t *testing.T, f *reservationFixture) (string, string) {
	t.Helper()
	ctx := context.Background()
	node := f.request.Payload.Agent
	object, err := catalog.Create(ctx, f.scope.Namespace, catalog.NewObject{Kind: "agent", Name: "different real role", Spec: catalog.Spec{Agent: &node}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := catalog.Publish(ctx, f.scope.Namespace, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	flow := catalog.Flow{Nodes: []catalog.Binding{{ID: "answer", AgentVersion: f.request.BaselineVersion, Task: "生成JSON"}, {ID: "review", AgentVersion: role.ID, Task: "核验JSON"}}}
	item, err := catalog.Create(ctx, f.scope.Namespace, catalog.NewObject{Kind: "workflow", Name: "multiple actual roles", Spec: catalog.Spec{Flow: &flow}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, f.scope.Namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	f.request.FlowObjectID = item.ID
	f.request.Payload.FlowVersion = version.ID
	f.request.Payload.BindingIDs = []string{"answer"}
	f.request.Budget = experience.Budget{Calls: 8, Tokens: 800}
	f.policy.Workflows = []string{item.ID}
	f.policy.Revision++
	f.request.PolicyRevision = f.policy.Revision
	f.savePolicy(t)
	return object.ID, role.ID
}
