package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompetitionRequiresSourcesBeforeModelCall(t *testing.T) {
	input := `{"question":"比较 Pangu 与 Eino、Coze","competitors":["Eino","Coze"],"dimensions":["受众","功能"]}`
	if _, err := ParseBrief(input, "competition"); err == nil {
		t.Fatal("没有资料的竞品调研仍可启动，重现 07593612 空报告")
	}
	input = strings.TrimSuffix(input, "}") + `,"public_queries":["Eino Coze 官方文档"]}`
	if _, err := ParseBrief(input, "competition"); err != nil {
		t.Fatal(err)
	}
}

func TestBriefOnlyRequiresGoalForPlanningAndDesign(t *testing.T) {
	for _, mode := range []string{"planning", "design"} {
		brief, err := ParseBrief(`{"question":"规划项目"}`, mode)
		if err != nil || brief.Question != "规划项目" || brief.Audience != "" {
			t.Fatalf("可选背景被拒绝或被编造: %+v %v", brief, err)
		}
	}
	for _, input := range []string{`{}`, `{"question":" "}`, `{"question":null}`} {
		if _, err := ParseBrief(input, "planning"); err == nil {
			t.Fatal("接受了空目标", input)
		}
	}
	for _, mode := range []string{"research", "competition"} {
		if _, err := ParseBrief(`{"question":"比较产品"}`, mode); err == nil {
			t.Fatal("放宽了调研来源或竞品要求", mode)
		}
	}
	data, _ := json.Marshal(map[string]string{"question": "规划", "context": strings.Repeat("a", 16*1024+1)})
	if _, err := ParseBrief(string(data), "planning"); err == nil {
		t.Fatal("放宽了背景大小限制")
	}
}

func TestBriefListErrorsIdentifyFieldAndItem(t *testing.T) {
	fields := map[string]string{
		"public_queries": "允许公开发送的搜索词", "urls": "授权公开网址",
		"documents": "授权项目相对文件", "competitors": "竞品候选", "dimensions": "比较维度",
	}
	for field, label := range fields {
		t.Run(field, func(t *testing.T) {
			brief := map[string]any{"question": "规划", "audience": "开发者", "market": "Agent", "time_scope": "最近半年", "context": "无", "constraints": "易用性"}
			for _, value := range []string{"", " \t\n", strings.Repeat("a", 2049)} {
				brief[field] = []string{"有效项", value}
				data, _ := json.Marshal(brief)
				_, err := ParseBrief(string(data), "planning")
				if err == nil || !strings.Contains(err.Error(), label+"第 2 项") {
					t.Fatalf("未定位非法列表项: %v", err)
				}
			}
			brief[field] = []string{strings.Repeat("a", 2048)}
			data, _ := json.Marshal(brief)
			if _, err := ParseBrief(string(data), "planning"); err != nil {
				t.Fatal("拒绝合法边界", err)
			}
			brief[field] = make([]string, 11)
			data, _ = json.Marshal(brief)
			if _, err := ParseBrief(string(data), "planning"); err == nil || !strings.Contains(err.Error(), label+"最多 10 项") {
				t.Fatal("未定位过长列表", err)
			}
		})
	}
}
