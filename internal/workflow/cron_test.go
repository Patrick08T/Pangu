package workflow

import (
	"strings"
	"testing"
)

func TestCronValidation(t *testing.T) {
	for _, expression := range []string{"0 2 * * *", "*/5 * * * *"} {
		c := Cron{Name: "daily", Expression: expression, Workflow: definition()}
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, expression := range []string{"", "@every 1m", "0 0 2 * * *", "0 30 * * *", "CRON_TZ=Asia/Shanghai 0 2 * * *"} {
		c := Cron{Name: "daily", Expression: expression, Workflow: definition()}
		if err := c.Validate(); err == nil {
			t.Fatalf("accepted %q", expression)
		}
	}
}

func TestCronDisplayNameAcceptsChineseWithoutControlCharacters(t *testing.T) {
	for _, name := range []string{"每日产品调研", "项目 A 的例行检查"} {
		if err := (Cron{Name: name, Expression: "0 0 * * *", Workflow: definition()}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"", "  ", "换行\n名称", strings.Repeat("名", 43)} {
		if err := (Cron{Name: name, Expression: "0 0 * * *", Workflow: definition()}).Validate(); err == nil {
			t.Fatalf("接受无效计划名 %q", name)
		}
	}
}
