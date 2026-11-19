package research

import (
	"context"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestReplayReadsFrozenDocumentsOnly(t *testing.T) {
	const raw = `{"brief":{"question":"总结公开资料","documents":["release.txt"]},"documents":["报告：收入增长 12%。"]}`
	input := workflow.NodeInput{Input: raw, Workspace: "host", Node: workflow.Node{Role: "product_manager", Mode: "research", Executor: "eino"}}
	if _, _, err := NewReplay(input); err == nil {
		t.Fatal("不能传入宿主工作区")
	}
	input.Workspace = ""
	session, brief, err := NewReplay(input)
	if err != nil || strings.Contains(brief, "12%") {
		t.Fatalf("err=%v brief=%s", err, brief)
	}
	for _, tc := range []struct{ name, args string }{
		{"read_web", `{"source_id":"d0"}`},
		{"search_public", `{"query_index":0}`},
		{"read_parent_report", `{"parent_id":"x","page":0}`},
		{"read_project", `{"document_index":1}`},
	} {
		if _, err := session.dispatch(context.Background(), tc.name, tc.args); err == nil {
			t.Fatalf("%s must fail", tc.name)
		}
	}
	got, err := session.dispatch(context.Background(), "read_project", `{"document_index":0}`)
	if err != nil || !strings.Contains(got, "12%") || !session.sources["d0"].Read || session.sources["d0"].SHA256 == "" {
		t.Fatalf("err=%v output=%s", err, got)
	}
	if len(session.Tools().Tools) != 2 {
		t.Fatal("回放只能读取冻结资料和登记报告")
	}
}
