package approval

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func TestEvidenceApprovalRequiresNewMaterial(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	input := structuredInput(project, "补充配置差异处置")
	input.ReviewPurpose = "evidence"
	gate, err := Request(context.Background(), actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	decision := Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: acceptedItems()}
	if _, err = Decide(context.Background(), actor, gate.ID, decision); !errors.Is(err, ErrInvalid) {
		t.Fatalf("没有新增材料仍重复复验: %v", err)
	}
	decision.Decision, decision.Reason, decision.Items = "changes_requested", "新增处置报告已放入平台证据目录，请核对版本差异", []workflow.ReviewAnswer{}
	if _, err = Decide(context.Background(), actor, gate.ID, decision); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceSelectionWithoutMaterialCannotRecheck(t *testing.T) {
	r := workflow.EvidenceReview([]workflow.EvidenceBlocker{{ID: "gap", Owner: "QA", Reason: "版本不一致", ResolutionCondition: "提供生效记录"}})
	d := Decision{Decision: "changes_requested", Items: []workflow.ReviewAnswer{{ItemID: "gap", Decision: "revise", SelectedOptionIDs: []string{"clarify"}}}}
	if validateGateDecision(&Gate{Purpose: "evidence", Review: &r}, d) == nil {
		t.Fatal("只切换选项仍可空转复验")
	}
}

func TestEvidenceRepairPayloadFollowsApprovalRetention(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	gate, err := Request(ctx, actor.Namespace, request, run, structuredInput(project, "范围"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "INSERT INTO pangu.evidence_repairs(id,approval_id,subject_hash,body,status,created_by) VALUES($1,$2,'hash','{}','prepared',$3)", uuid.NewString(), gate.ID, actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, "UPDATE pangu.approvals SET subject=NULL WHERE id=$1", gate.ID)
	if err != nil {
		t.Fatal(err)
	}
	var purged bool
	if err = conn.QueryRow(ctx, "SELECT body IS NULL FROM pangu.evidence_repairs WHERE approval_id=$1", gate.ID).Scan(&purged); err != nil || !purged {
		t.Fatal("修正报告未随源审批清理", err)
	}
	if _, err = Get(ctx, actor.Namespace, gate.ID); err != nil {
		t.Fatal("清理后历史不可查", err)
	}
}
