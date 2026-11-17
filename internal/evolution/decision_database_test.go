//go:build integration

package evolution

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
)

func TestDecideUsesStoredEvidenceAndLiveEnvironment(t *testing.T) {
	f := newReservationFixture(t)
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := f.service.Decide(ctx, f.scope, intent.ID, intent.Generation)
	if err != nil || decision.Outcome != "insufficient" {
		t.Fatal("无真实成对结果不得通过", decision, err)
	}
	disabled, err := f.service.RegisterEnvironment(ctx, f.scope, Environment{ID: f.request.EnvironmentID, ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", Enabled: false, ConnectionVersion: f.request.Payload.Agent.ConnectionVersion}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Enabled {
		t.Fatal("环境未停用")
	}
	if _, err = f.service.Decide(ctx, f.scope, intent.ID, intent.Generation); err == nil {
		t.Fatal("用冻结实验冒充current绕过实时环境")
	}
}

func TestPromoteRejectsUnprovenExperiment(t *testing.T) {
	f := newReservationFixture(t)
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Promote(ctx, f.scope, intent.ID, intent.Generation); err == nil {
		t.Fatal("没有可信passed仍发布")
	}
}

func TestDecideAndPromoteRealStoredPairsAreIdempotent(t *testing.T) {
	f, intent := completedDecisionFixture(t)
	ctx := context.Background()
	decision, err := f.service.Decide(ctx, f.scope, intent.ID, intent.Generation)
	assertPassedDecision(t, decision, err)
	published, err := f.service.Promote(ctx, f.scope, intent.ID, intent.Generation)
	if err != nil || published.Publication == nil || published.Publication.AgentID == "" || published.State != "switching" {
		t.Fatal("未原子发布进入switching", published, err)
	}
	retry, err := f.service.Promote(ctx, f.scope, intent.ID, intent.Generation)
	if err != nil || retry.Publication == nil || retry.Publication.AgentID != published.Publication.AgentID {
		t.Fatal("发布重试不是幂等", retry, err)
	}
	if _, err = catalog.ReadVersion(ctx, f.scope.Namespace, published.Publication.AgentID); err == nil {
		t.Fatal("自动版本泄露到共享目录")
	}
	object, err := catalog.Get(ctx, f.scope.Namespace, f.request.AgentObjectID)
	if err != nil || object.Spec.Agent.Instruction == f.request.Payload.Agent.Instruction {
		t.Fatal("自动发布覆盖草稿", object, err)
	}
}

func assertPassedDecision(t *testing.T, decision evaluation.ExperimentDecision, err error) {
	t.Helper()
	if err != nil || decision.Outcome != "passed" {
		t.Fatal("真实checker成对证据未通过", decision, err)
	}
}
