package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestApprovalBindsFullReportAndRejectsChangedBody(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	report, node := uuid.NewString(), uuid.NewString()
	body := withReview(`{"ready":true,"evidence":["` + strings.Repeat("完整审批任务", 3000) + `"]}`)
	hash := sha256.Sum256([]byte(body))
	_, err = conn.Exec(ctx, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','base','test',$3,true)`, run, project, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence) VALUES($1,$2,$3,'完整计划',$4,'{}','[]')`, report, run, node, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(map[string]any{"text": `{"ready":true}`, "run_id": node, "report_ids": []string{report}, "report_sha256": hex.EncodeToString(hash[:])})
	var parent workflow.NodeOutput
	_ = json.Unmarshal(encoded, &parent)
	input := workflow.NodeInput{ProjectID: project, Node: workflow.Node{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}, Approval: &workflow.Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}}, Parents: map[string]workflow.NodeOutput{"plan": parent}}
	gate, err := Request(ctx, actor.Namespace, request, run, input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gate.Subject), strings.Repeat("完整审批任务", 3000)) {
		t.Fatal("审批只绑定引用，没有展示完整计划")
	}
	if _, err = Decide(ctx, actor, gate.ID, Decision{SubjectHash: gate.SubjectHash, Decision: "approved", Items: acceptedItems()}); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.research_reports SET body=$2 WHERE id=$1", report, []byte(`{"ready":true,"tasks":["不同范围"]}`)); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); err == nil {
		t.Fatal("批准后被替换的报告仍可执行")
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.research_reports SET body=$2 WHERE id=$1", report, []byte(body)); err != nil {
		t.Fatal(err)
	}
	if err = Consume(ctx, run, input, gate.ID); err != nil {
		t.Fatalf("原报告恢复后无法执行: %v", err)
	}
}
