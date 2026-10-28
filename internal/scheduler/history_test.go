package scheduler

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestTaskTitleUsesGoalWithoutExposingCredentials(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`{"question":"规划任务入口","context":"内部背景"}`, "规划任务入口"},
		{"检查\n 项目", "检查 项目"},
		{"", "未命名任务"},
		{"检查 api_key=private-value", "含敏感信息的任务"},
	} {
		if got := taskTitle(workflow.Definition{Input: tc.input}); got != tc.want {
			t.Errorf("title=%q want=%q", got, tc.want)
		}
	}
	title := taskTitle(workflow.Definition{Input: strings.Repeat("目标", 200)})
	if !utf8.ValidString(title) || len(title) > 243 {
		t.Fatal("任务标题截断无效")
	}
}
