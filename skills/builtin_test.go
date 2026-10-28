package skills

import (
	"strings"
	"testing"
)

func TestBuiltinSkillIsBundledAndModeScoped(t *testing.T) {
	build, err := Read("build")
	if err != nil || !strings.Contains(build.Text, "Workflow 构建与优化") || !strings.Contains(build.Text, "上下文、状态与执行契约") || len(build.Digest) != 64 {
		t.Fatalf("内置技能不完整: %+v %v", build, err)
	}
	optimize, err := Read("optimize")
	if err != nil || !strings.Contains(optimize.Text, "评分隔离") || build.Digest == optimize.Digest {
		t.Fatal("优化资料未按模式加载", err)
	}
	if _, err := Read("../../personal"); err == nil {
		t.Fatal("接受任意技能路径")
	}
}
