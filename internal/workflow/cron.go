package workflow

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/robfig/cron/v3"
)

type Cron struct {
	Name       string     `json:"name"`
	Expression string     `json:"expression"`
	Workflow   Definition `json:"workflow"`
}

func (c Cron) Validate() error {
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 128 || !utf8.ValidString(c.Name) || strings.IndexFunc(c.Name, unicode.IsControl) >= 0 {
		return fmt.Errorf("计划名称不能为空、超过 128 字节或包含控制字符")
	}
	if _, err := Preview(c.Expression, time.Now()); err != nil {
		return err
	}
	if err := c.Workflow.ValidateInput(); err != nil {
		return err
	}
	_, err := c.Workflow.Layers()
	return err
}

func Preview(expression string, now time.Time) ([]time.Time, error) {
	if len(strings.Fields(expression)) != 5 {
		return nil, fmt.Errorf("Cron 必须使用 UTC 五字段表达式")
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("Cron 表达式不合法: %w", err)
	}
	out := []time.Time{}
	next := now.UTC()
	for range 5 {
		next = schedule.Next(next)
		if next.IsZero() {
			return nil, fmt.Errorf("Cron 在可计算范围内没有下一次触发")
		}
		out = append(out, next)
	}
	return out, nil
}
