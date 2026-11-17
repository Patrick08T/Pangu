package factory

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestFactoryAppliesReusableRoleAcrossStepsWithoutCreatingDuplicates(t *testing.T) {
	ctx, ns, s := setup(t)
	r, err := Parse(testReply())
	if err != nil {
		t.Fatal(err)
	}
	a := r.Proposal.Agents[0]
	item, err := catalog.Create(ctx, ns, catalog.NewObject{Kind: "agent", Name: a.Name, Spec: catalog.Spec{Agent: a.Node}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	in := prepareTest(t, ctx, ns, s)
	r.Proposal.Agents = []catalog.AgentChange{{Key: "writer", Name: item.Name, VersionID: v.ID}}
	r.Proposal.Steps = []catalog.BundleStep{{ID: "first", Agent: "writer"}, {ID: "second", Agent: "writer", DependsOn: []string{"first"}}}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err = Complete(ctx, ns, s.ID, in.ID, string(data), ""); err != nil {
		t.Fatal(err)
	}
	s, err = Get(ctx, ns, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Apply(ctx, ns, s.ID, in.ID, s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := catalog.List(ctx, ns, "agent", 0, nil)
	if err != nil || len(roles) != 1 || roles[0].ID != item.ID || roles[0].Revision != item.Revision {
		t.Fatal("复用候选创建或修改了角色", err)
	}
	if out.Workflow == nil || len(out.Workflow.Spec.Flow.Nodes) != 2 {
		t.Fatal("未生成独立执行步骤")
	}
	for _, step := range out.Workflow.Spec.Flow.Nodes {
		if step.AgentVersion != v.ID {
			t.Fatal("步骤未绑定同一已有版本")
		}
	}
}

func TestFactoryRejectsOversizedRoleContextWithoutTruncation(t *testing.T) {
	in := Input{ID: uuid.NewString(), Revision: 1, Mode: "build", Message: "复用角色", ConnectionVersion: uuid.NewString()}
	data := Context{
		Connections: []catalog.ConnectionChoice{{ID: in.ConnectionVersion, Provider: "codex"}},
		Agents: []catalog.RunnableChoice{{Version: catalog.Version{Spec: catalog.Spec{
			Agent: &workflow.Node{Instruction: strings.Repeat("x", 180001)},
		}}}},
	}
	d, _, err := Definition(Session{ProjectID: uuid.NewString()}, in, data)
	if !errors.Is(err, catalog.ErrInvalid) || len(d.Nodes) != 0 {
		t.Fatal("角色上下文超限后仍提交了不完整候选范围", err)
	}
}
