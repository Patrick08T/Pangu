package research

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type reportArgs struct {
	Title     string   `json:"title"`
	Markdown  string   `json:"markdown"`
	SourceIDs []string `json:"source_ids"`
	Tasks     []Task   `json:"tasks,omitempty"`
}

func sections(mode string) []string {
	base := []string{"结论", "证据与推断", "未知项"}
	switch mode {
	case "research":
		return append(base, "调研范围与方法", "发现与建议")
	case "competition":
		return append(base, "竞品矩阵", "定位差异")
	case "planning":
		return append(base, "PRD", "优先级与路线图", "成功指标")
	case "design":
		return append(base, "用户流程", "输入输出与异常边界", "任务与依赖", "验收条件")
	}
	return base
}

func (s *Session) register(a reportArgs) (string, error) {
	if s.report != nil {
		return "", errors.New("每个节点只登记一份最终报告")
	}
	if len(a.Title) == 0 || len(a.Title) > 256 || strings.ContainsAny(a.Title, "\r\n") || !utf8.ValidString(a.Title) || diagnostic.Sensitive([]byte(a.Title)) || !utf8.ValidString(a.Markdown) || len(a.Markdown) > 1024*1024-16384 || diagnostic.Sensitive([]byte(a.Markdown)) {
		return "", errors.New("报告标题、大小或敏感信息校验失败")
	}
	for _, section := range sections(s.input.Node.Mode) {
		if !strings.Contains("\n"+a.Markdown+"\n", "\n## "+section+"\n") {
			return "", fmt.Errorf("报告缺少二级标题：%s", section)
		}
	}
	if strings.Contains(strings.ToLower(a.Markdown), "http:") || strings.Contains(strings.ToLower(a.Markdown), "https:") || strings.Contains(a.Markdown, "](") || strings.Contains(a.Markdown, "<") || strings.Contains(a.Markdown, "![") || regexp.MustCompile(`\]\s*:`).MatchString(a.Markdown) {
		return "", errors.New("报告正文使用 [来源ID] 引用，不接受自编链接或 HTML")
	}
	if len(a.SourceIDs) == 0 || len(a.SourceIDs) > 30 {
		return "", errors.New("报告必须引用 1 至 30 个实际读取的来源")
	}
	known := map[string]bool{}
	appendix := "\n\n## 来源记录（平台生成）\n"
	if s.input.Node.Mode == "design" {
		tasks, err := taskAppendix(a.Tasks)
		if err != nil {
			return "", err
		}
		appendix = tasks + appendix
	} else if len(a.Tasks) > 0 {
		return "", errors.New("只有方案设计模式接受 tasks")
	}
	for _, id := range a.SourceIDs {
		source, ok := s.sources[id]
		if !ok || !source.Read || known[id] {
			return "", errors.New("报告来源未读取、不存在或重复")
		}
		known[id] = true
		appendix += fmt.Sprintf("\n- %s %s；获取时间：%s；地址：%s；摘要：%s；上游产物：%s", literal("["+id+"]"), literal(source.Title), literal(source.RetrievedAt.Format("2006-01-02T15:04:05Z")), literal(source.URL), literal(source.SHA256), literal(source.ArtifactID))
	}
	for _, match := range regexp.MustCompile(`\[([A-Za-z][A-Za-z0-9_-]*)\]`).FindAllStringSubmatch(a.Markdown, -1) {
		if !known[match[1]] {
			return "", errors.New("正文引用未包含在来源清单")
		}
	}
	if s.input.Node.Mode == "research" && len(known) == 1 && known["input"] {
		return "", errors.New("调研至少需要一个实际读取的外部或项目来源")
	}
	// 引用只作为文本展示，转义全部括号，阻止引用定义/图片在其他 Markdown 阅读器中激活。
	body := strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]").Replace(a.Markdown)
	if len(body)+len(appendix) > 1024*1024 {
		return "", errors.New("生成后的报告超过 1 MiB")
	}
	s.report = &workflow.Report{ID: uuid.NewString(), Title: a.Title, Markdown: body + appendix}
	return "报告已生成，待节点成功后原子保存。产物 ID：" + s.report.ID, nil
}

func literal(value string) string {
	return "`" + strings.ReplaceAll(strings.Join(strings.Fields(value), " "), "`", "ˋ") + "`"
}
