package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestCodexLongPlanBecomesBoundedReportReference(t *testing.T) {
	setupCodex(t)
	input := codexInput("report-fixture")
	config := `{"id":"plan","executor":"codex","model":"report-fixture","instruction":"形成完整任务计划","read_only":true,"output_schema":{"type":"object","properties":{"ready":{"type":"boolean"},"summary":{"type":"string"}},"required":["ready","summary"]},"report_schema":{"type":"object","properties":{"ready":{"type":"boolean"},"summary":{"type":"string"},"tasks":{"type":"array","items":{"type":"string"}}},"required":["ready","summary","tasks"]}}`
	if err := json.Unmarshal([]byte(config), &input.Node); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"ready": true, "summary": "统一执行计划", "tasks": []string{strings.Repeat("任务验收与真实验证；", 3000)}})
	input.Input = string(raw)
	out, err := Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("真实执行入口拒绝完整长计划: %v", err)
	}
	encoded, _ := json.Marshal(out)
	if len(encoded) > 16384 || len(out.Reports) != 1 || len(out.ReportIDs) != 1 || out.Reports[0].Markdown != string(raw) {
		t.Fatalf("长计划未完整保存或仍进入节点信封: bytes=%d reports=%d", len(encoded), len(out.Reports))
	}
	var summary map[string]any
	if json.Unmarshal([]byte(out.Text), &summary) != nil || summary["ready"] != true || summary["tasks"] != nil {
		t.Fatal("摘要丢失控制字段或重复携带完整任务")
	}
	if err = workflow.ValidateOutput(input.Node, &out); err != nil {
		t.Fatal(err)
	}
}

func TestFullReportRejectsSchemaErrorsAndSensitiveContent(t *testing.T) {
	shape := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"ready": {Type: "boolean"}, "summary": {Type: "string"}}, Required: []string{"ready", "summary"}}
	node := workflow.Node{ReportSchema: shape, OutputSchema: shape}
	for _, body := range []string{`{"ready":"yes","summary":"错误类型"}`, `{"ready":true}`, `{"ready":true,"summary":"api_key=private-value-123"}`, strings.Repeat("x", 1024*1024+1)} {
		out, err := reportOutput(node, workflow.NodeOutput{Text: body})
		if err == nil || len(out.ReportIDs) != 0 {
			t.Fatal("无效完整报告被发布")
		}
	}
}

func TestReportProjectionKeepsControlStringsExact(t *testing.T) {
	shape := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"decision": {Type: "string"}, "summary": {Type: "string"}}}
	decision := strings.Repeat("控制字段", 300)
	raw, _ := json.Marshal(map[string]string{"decision": decision, "summary": decision})
	out, err := reportOutput(workflow.Node{ID: "plan", ReportSchema: shape, OutputSchema: shape}, workflow.NodeOutput{Text: string(raw)})
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]string
	if json.Unmarshal([]byte(out.Text), &summary) != nil || summary["decision"] != decision || len(summary["summary"]) > 1024 || out.Reports[0].Markdown != string(raw) {
		t.Fatal("摘要裁剪影响控制字段或完整报告")
	}
}
