package approval

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func TestApprovalNeverExposesSensitiveParentOutput(t *testing.T) {
	actor, project, run, request := approvalFixture(t)
	input := structuredInput(project, "范围")
	input.Parents["source"] = workflow.NodeOutput{Text: "api_key=test-secret-value"}
	if _, err := Request(context.Background(), actor.Namespace, request, run, input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("将敏感输出转入审批正文: %v", err)
	}
	gates, err := List(context.Background(), actor.Namespace, run)
	if err != nil || len(gates) != 0 {
		t.Fatalf("敏感审批被保存: %+v %v", gates, err)
	}
}
