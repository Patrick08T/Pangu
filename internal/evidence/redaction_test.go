package evidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandEvidenceRedactsWithoutBlockingOtherSources(t *testing.T) {
	b := &Bundle{}
	body := `{"command":"rg 'PRIVATE KEY-----' docs", "output":"Authorization: Bearer example-secret-value"}`
	if err := b.add("run", "actions", "命令", "actions.json", body); err != nil {
		t.Fatal("安全扫描命令不应阻断全部证据", err)
	}
	if len(b.Files) != 1 || strings.Contains(b.Files[0].Body, "example-secret-value") || !json.Valid([]byte(b.Files[0].Body)) {
		t.Fatal("脱敏交接泄漏凭据或破坏 JSON")
	}
	if !strings.Contains(b.Files[0].Body, "redacted_text") {
		t.Fatal("没有声明脱敏，不可冒充完整原文")
	}
	if err := b.add("run", "report", "报告", "report.txt", body); err == nil {
		t.Fatal("不得默默改写有哈希身份的报告")
	}
}
