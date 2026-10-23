package workflow

import "testing"

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
