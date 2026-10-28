package research

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"strings"
	"testing"
)

func qualityReport(mode string) reportArgs {
	a := reportArgs{Title: "产品分析", Markdown: "## " + strings.Join(sections(mode), "\n有内容\n## ") + "\n有内容 [input]", SourceIDs: []string{"input"},
		Claims: []Claim{{ID: "c1", Statement: "资料未知", Kind: "unknown", Scope: "当前产品", Uncertainty: "未取得资料"}}}
	if mode == "design" {
		a.Tasks = []Task{{ID: "t1", Title: "验证", Scope: "当前项目", Deliverable: "验收记录", Acceptance: "通过验收"}}
	}
	return a
}

func TestEvidenceGateRejectsEmptyResearchAndIgnoredSources(t *testing.T) {
	for _, mode := range []string{"research", "competition", "planning", "design"} {
		t.Run(mode, func(t *testing.T) {
			in := researchInput()
			in.Node.Mode = mode
			in.Input = `{"question":"对比并规划","public_queries":["Eino Coze 官方"],"documents":["README.md"],"competitors":["Eino","Coze"],"dimensions":["功能"]}`
			s, err := New(in, nil)
			if err != nil {
				t.Fatal(err)
			}
			a := qualityReport(mode)
			if _, err := s.register(a); err == nil {
				t.Fatal("没有读取任何资料却登记成功")
			}
			s.sources["web"] = Source{ID: "web", URL: "https://example.com/", Read: true}
			s.sourceText["web"] = "官方产品说明"
			a.SourceIDs = append(a.SourceIDs, "web")
			a.Claims = []Claim{{ID: "c1", Statement: "产品说明", Kind: "fact", Scope: "当前版本", Uncertainty: "体验未实测", Evidence: []Evidence{{SourceID: "web", Quote: "官方产品说明", SourceDate: "unknown"}}}}
			if _, err := s.register(a); err == nil {
				t.Fatal("忽略授权项目资料仍能登记")
			}
			s.sources["d0"] = Source{ID: "d0", Read: true}
			s.sourceText["d0"] = "项目支持独立调度"
			a.SourceIDs = append(a.SourceIDs, "d0")
			a.Claims = append(a.Claims, Claim{ID: "c2", Statement: "支持独立调度", Kind: "fact", Scope: "当前 Git 基准", Uncertainty: "未复测", Evidence: []Evidence{{SourceID: "d0", Quote: "项目支持独立调度", SourceDate: "unknown"}}})
			if _, err := s.register(a); err != nil {
				t.Fatal("有网页及项目原文仍被拒绝", err)
			}
		})
	}
}

func TestReadSourcesWithoutFactsDoNotPassCompetition(t *testing.T) {
	in := researchInput()
	in.Node.Mode = "competition"
	in.Input = `{"question":"比较产品","documents":["README.md"],"competitors":["产品"],"dimensions":["功能"]}`
	s, _ := New(in, nil)
	s.sources["d0"] = Source{ID: "d0", Read: true}
	s.sourceText["d0"] = "产品说明"
	a := qualityReport("competition")
	a.SourceIDs = append(a.SourceIDs, "d0")
	if _, err := s.register(a); err == nil {
		t.Fatal("读了来源却只交付未知项仍被接受")
	}
}

func TestUnavailableToolsAreNotOfferedToModel(t *testing.T) {
	s, err := New(researchInput(), nil)
	if err != nil {
		t.Fatal(err)
	}
	tools := s.Tools().Tools
	if len(tools) != 1 {
		t.Fatalf("没有资料却暴露 %d 个工具", len(tools))
	}
	info, err := tools[0].Info(context.Background())
	if err != nil || info.Name != "register_report" {
		t.Fatal(info, err)
	}
}

func TestReportRejectsEmptySectionBodies(t *testing.T) {
	s, _ := New(researchInput(), nil)
	a := qualityReport("planning")
	a.Markdown = "## " + strings.Join(sections("planning"), "\n## ") + "\n"
	if _, err := s.register(a); err == nil {
		t.Fatal("仅标题的空报告被接受")
	}
	if _, err := s.Finish(workflow.NodeOutput{}); err == nil {
		t.Fatal("空报告被标记完成")
	}
}

func TestRegisteredReportPreventsFurtherExternalCalls(t *testing.T) {
	calls := 0
	s, _ := New(researchInput(), func(context.Context, string) ([]SearchResult, error) {
		calls++
		return nil, nil
	})
	s.brief.PublicQueries = []string{"公开资料"}
	s.report = &workflow.Report{Title: "已完成"}
	if _, err := s.invoke(context.Background(), "search_public", `{"query_index":0}`); err != nil || calls != 0 || s.calls != 0 || len(s.evidence) != 0 {
		t.Fatalf("报告完成后仍访问外部服务: calls=%d used=%d err=%v", calls, s.calls, err)
	}
}
