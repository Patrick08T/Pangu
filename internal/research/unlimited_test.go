package research

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestToolCallsBeyondTenStillValidateAuthority(t *testing.T) {
	session, err := New(researchInput(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 12 {
		_, err = session.invoke(context.Background(), "read_project", `{"document_index":0}`)
		if err == nil || workflow.FailureKind(err) == "tool_limit" {
			t.Fatalf("应继续校验资料授权，不应因次数拒绝或放开授权：%v", err)
		}
	}
}
