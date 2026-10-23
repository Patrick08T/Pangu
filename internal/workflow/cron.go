package workflow

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

type Cron struct {
	Name       string     `json:"name"`
	Expression string     `json:"expression"`
	Workflow   Definition `json:"workflow"`
}

func (c Cron) Validate() error {
	if !identifier.MatchString(c.Name) {
		return fmt.Errorf("Cron 名称不合法")
	}
	if len(strings.Fields(c.Expression)) != 5 {
		return fmt.Errorf("Cron 必须使用 UTC 五字段表达式")
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(c.Expression); err != nil {
		return fmt.Errorf("Cron 表达式不合法: %w", err)
	}
	_, err := c.Workflow.Layers()
	return err
}
