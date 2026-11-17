package approval

import (
	"github.com/Patrick08T/Pangu/internal/workflow"
	"testing"
)

func TestInheritedInputKeepsContentButRebindsReportIdentity(t *testing.T) {
	a := workflow.NodeInput{Node: workflow.Node{ID: "build"}, Parents: map[string]workflow.NodeOutput{"plan": {Text: "完整获批计划", RunID: "old", ReportIDs: []string{"old-report"}, ReportSHA256: "same"}}}
	b := workflow.NodeInput{Node: a.Node, Parents: map[string]workflow.NodeOutput{"plan": {Text: "完整获批计划", RunID: "new", ReportIDs: []string{"new-report"}, ReportSHA256: "same"}}, ApprovalRound: 1}
	if !sameInheritedInput(a, b) {
		t.Fatal("复制报告的 ID 使合法授权无法继承")
	}
	changed := b.Parents["plan"]
	changed.Text = "替换计划"
	b.Parents["plan"] = changed
	if sameInheritedInput(a, b) {
		t.Fatal("继承授权接受了未批准正文")
	}
	b.Parents["plan"] = a.Parents["plan"]
	b.Node.Instruction = "改变实施范围"
	if sameInheritedInput(a, b) {
		t.Fatal("继承授权接受了未批准节点")
	}
}
