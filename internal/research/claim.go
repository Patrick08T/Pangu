package research

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/cloudwego/eino/schema"
	"regexp"
	"strings"
	"time"
)

type Evidence struct {
	SourceID   string `json:"source_id"`
	Quote      string `json:"quote"`
	SourceDate string `json:"source_date"`
}
type Claim struct {
	ID          string     `json:"id"`
	Statement   string     `json:"statement"`
	Kind        string     `json:"kind"`
	Scope       string     `json:"scope"`
	Uncertainty string     `json:"uncertainty"`
	Conflicts   []string   `json:"conflicts"`
	Evidence    []Evidence `json:"evidence"`
}
type StructuredReport struct {
	Mode    string   `json:"mode"`
	Tasks   []Task   `json:"tasks"`
	Claims  []Claim  `json:"claims"`
	Sources []Source `json:"sources"`
}

func (s *Session) claims(claims []Claim, sourceIDs []string) (string, error) {
	if len(claims) < 1 || len(claims) > 30 {
		return "", errors.New("报告必须登记 1–30 项关键结论及依据")
	}
	allowed := map[string]bool{}
	for _, id := range sourceIDs {
		allowed[id] = true
	}
	ids := map[string]bool{}
	out := "\n\n## 关键结论与依据（平台校验）\n\n引文仅校验来自已读取内容，是否支持结论尚需人工判定。资料日期由模型标注，不等于平台获取日期；unknown 表示日期未知。\n"
	for _, claim := range claims {
		if ids[claim.ID] || !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`).MatchString(claim.ID) {
			return "", errors.New("结论 ID 无效或重复")
		}
		ids[claim.ID] = true
		if err := s.checkClaim(claim, allowed); err != nil {
			return "", err
		}
		out += "\n- " + literal(claim.ID+"："+claim.Statement) + "；类别：" + literal(claim.Kind) + "；适用范围：" + literal(claim.Scope) + "；待核验：" + literal(claim.Uncertainty) + "\n"
		for _, issue := range claim.Conflicts {
			out += "  - 冲突：" + literal(issue) + "\n"
		}
		for _, item := range claim.Evidence {
			out += fmt.Sprintf("  - 依据 %s；原文：%s；资料日期（待核对）：%s\n", literal(item.SourceID), literal(item.Quote), literal(item.SourceDate))
		}
	}
	return out, nil
}

func (s *Session) checkClaim(claim Claim, allowed map[string]bool) error {
	if claim.Kind != "fact" && claim.Kind != "inference" && claim.Kind != "unknown" {
		return errors.New("结论类型必须为 fact/inference/unknown")
	}
	if len(claim.Evidence) > 5 || len(claim.Conflicts) > 5 || claim.Kind == "fact" && (len(claim.Evidence) == 0 || len(claim.Conflicts) > 0) {
		return errors.New("事实必须有依据且不能隐瞒冲突；有冲突请标为推断或未知")
	}
	for _, value := range append([]string{claim.Statement, claim.Scope, claim.Uncertainty}, claim.Conflicts...) {
		if strings.TrimSpace(value) == "" || len(value) > 2048 {
			return errors.New("结论、范围、待核验或冲突字段必须为 1–2048 字节")
		}
	}
	for _, item := range claim.Evidence {
		if !allowed[item.SourceID] || !s.sources[item.SourceID].Read || strings.TrimSpace(item.Quote) == "" || len(item.Quote) > 1024 || !strings.Contains(s.sourceText[item.SourceID], item.Quote) {
			return errors.New("结论引文必须匹配本次实际读取的原文，且最多 1024 字节")
		}
		if item.SourceDate != "unknown" {
			if _, err := time.Parse("2006-01-02", item.SourceDate); err != nil {
				return errors.New("资料日期必须为 YYYY-MM-DD 或 unknown")
			}
		}
	}
	raw, err := json.Marshal(claim)
	if err != nil || diagnostic.Sensitive(raw) {
		return errors.New("结论含无效或敏感信息")
	}
	return nil
}

func claimParameters() *schema.ParameterInfo {
	text := func(description string) *schema.ParameterInfo {
		return &schema.ParameterInfo{Type: schema.String, Required: true, Desc: description}
	}
	return &schema.ParameterInfo{Type: schema.Array, Required: true, Desc: "1–30 项关键结论；引文必须出现在实际读取内容中；冲突不能声明为确定事实", ElemInfo: &schema.ParameterInfo{Type: schema.Object, SubParams: map[string]*schema.ParameterInfo{
		"id": text("唯一结论 ID"), "statement": text("关键结论"), "kind": text("fact/inference/unknown"), "scope": text("适用市场、对象、范围"), "uncertainty": text("证据局限和仍需人工核验的问题"),
		"conflicts": {Type: schema.Array, Required: true, ElemInfo: &schema.ParameterInfo{Type: schema.String}},
		"evidence":  {Type: schema.Array, Required: true, ElemInfo: &schema.ParameterInfo{Type: schema.Object, SubParams: map[string]*schema.ParameterInfo{"source_id": text("实际读取且列入 source_ids 的来源"), "quote": text("不超过 1024 字节的原文摘录"), "source_date": text("资料日期 YYYY-MM-DD，无法确认填 unknown；不可用获取时间代替")}}},
	}}}
}
