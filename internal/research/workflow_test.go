package research

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func researchInput() workflow.NodeInput {
	b := Brief{Question: "规划团队任务台", Audience: "小型研发团队", Market: "中国", TimeScope: "2026 年", Context: "现状为手工登记，每周重复整理", Constraints: "不新增专职人员"}
	raw, _ := json.Marshal(b)
	return workflow.NodeInput{Node: workflow.Node{ID: "pm", Executor: "eino", Model: "test", Instruction: "规划", Role: "product_manager", Mode: "planning"}, Input: string(raw)}
}

func TestToolsCannotExpandAuthorityAndShareCallLimit(t *testing.T) {
	in := researchInput()
	in.Node.Policy = &workflow.Policy{TimeoutSeconds: 180, ToolCalls: 2}
	s, err := New(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{`{"query":"机密项目资料"}`, `{"source_id":"https://example.com/secret"}`, `{"document_index":0}`} {
		_, err = s.invoke(context.Background(), "read_web", args)
		if err == nil {
			t.Fatal("允许模型扩大权限")
		}
	}
	if s.calls != 2 {
		t.Fatalf("工具失败也应消耗次数，实际 %d", s.calls)
	}
}

func TestReportRejectsInventedSourcesAndRequiresModeSections(t *testing.T) {
	s, err := New(researchInput())
	if err != nil {
		t.Fatal(err)
	}
	args := reportArgs{Title: "任务台规划", Markdown: "## 结论\n建议 [input]\n## 证据与推断\n用户资料\n## 未知项\n待验证\n## PRD\n范围\n## 优先级与路线图\n先验证\n## 成功指标\n整理耗时", SourceIDs: []string{"fake"}}
	if _, err := s.register(args); err == nil {
		t.Fatal("接受虚构来源")
	}
	args.SourceIDs = []string{"input"}
	if _, err := s.register(args); err != nil {
		t.Fatal(err)
	}
	out, err := s.Finish(workflow.NodeOutput{})
	if err != nil || len(out.Reports) != 1 || len(out.ReportIDs) != 1 || !strings.Contains(out.Reports[0].Markdown, "获取时间") {
		t.Fatalf("报告缺少真实元数据: %+v %v", out, err)
	}
	if _, err := s.register(args); err == nil {
		t.Fatal("一个节点登记了多份报告")
	}
}

func TestReportTitleAndMarkdownCannotLeakOrInventLinks(t *testing.T) {
	s, err := New(researchInput())
	if err != nil {
		t.Fatal(err)
	}
	body := "## 结论\n建议 [input]\n## 证据与推断\n用户资料\n## 未知项\n待验证\n## PRD\n范围\n## 优先级与路线图\n先验证\n## 成功指标\n整理耗时"
	args := reportArgs{Title: "api_key=private-value-123", Markdown: body, SourceIDs: []string{"input"}}
	if _, err = s.register(args); err == nil {
		t.Fatal("标题泄漏了凭据")
	}
	args.Title = "规划"
	args.Markdown = body + "\n[input]: //evil.example/forged\n![input]"
	if _, err = s.register(args); err == nil {
		t.Fatal("允许伪造引用式链接或图片")
	}
}
