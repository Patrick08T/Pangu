package research

import (
	"errors"
	"regexp"
	"strings"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Scope       string   `json:"scope"`
	DependsOn   []string `json:"depends_on"`
	Deliverable string   `json:"deliverable"`
	Acceptance  string   `json:"acceptance"`
}

func taskAppendix(tasks []Task) (string, error) {
	if len(tasks) < 1 || len(tasks) > 32 {
		return "", errors.New("方案设计必须含 1 至 32 个可执行任务")
	}
	ids := map[string]bool{}
	for _, task := range tasks {
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`).MatchString(task.ID) || ids[task.ID] {
			return "", errors.New("任务 ID 不合法或重复")
		}
		ids[task.ID] = true
		for _, field := range []string{task.Title, task.Scope, task.Deliverable, task.Acceptance} {
			if strings.TrimSpace(field) == "" || len(field) > 4096 || diagnostic.Sensitive([]byte(field)) {
				return "", errors.New("任务必须包含无凭据的标题、范围、交付物和验收条件，每项最多 4 KiB")
			}
		}
	}
	for _, task := range tasks {
		seen := map[string]bool{}
		for _, dependency := range task.DependsOn {
			if !ids[dependency] || seen[dependency] {
				return "", errors.New("任务依赖不存在或重复")
			}
			seen[dependency] = true
		}
	}
	done := map[string]bool{}
	for len(done) < len(tasks) {
		before := len(done)
		for _, task := range tasks {
			ready := true
			for _, id := range task.DependsOn {
				ready = ready && done[id]
			}
			if ready {
				done[task.ID] = true
			}
		}
		if len(done) == before {
			return "", errors.New("任务依赖包含环")
		}
	}
	out := "\n\n## 可执行任务（平台校验）\n"
	for _, task := range tasks {
		out += "\n- " + literal(task.ID+"："+task.Title) + "\n  - 范围：" + literal(task.Scope) + "\n  - 前置：" + literal(strings.Join(task.DependsOn, ", ")) + "\n  - 交付：" + literal(task.Deliverable) + "\n  - 验收：" + literal(task.Acceptance) + "\n"
	}
	return out, nil
}
