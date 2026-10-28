package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSignedIssueRequiresExactRepositoryLabelAndBoundedContent(t *testing.T) {
	secret := strings.Repeat("s", 32)
	body := []byte(`{"action":"labeled","repository":{"full_name":"owner/repo"},"label":{"name":"pangu"},"issue":{"number":7,"title":"修复接口","body":"范围与验收","html_url":"https://github.com/owner/repo/issues/7"}}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	config := Config{Repository: "owner/repo", Label: "pangu"}
	if !Signature(body, sig, secret) || Signature(append(body, ' '), sig, secret) || Signature(body, "sha256=bad", secret) {
		t.Fatal("签名校验错误")
	}
	input, err := Input(body, config)
	if err != nil || !strings.Contains(input, "范围与验收") {
		t.Fatalf("输入 %s %v", input, err)
	}
	for _, wrong := range []string{strings.ReplaceAll(string(body), "owner/repo", "other/repo"), strings.ReplaceAll(string(body), `"pangu"`, `"other"`), strings.ReplaceAll(string(body), `"labeled"`, `"opened"`), string(body) + "{}"} {
		if _, err = Input([]byte(wrong), config); err == nil {
			t.Fatalf("接受无效事件 %s", wrong)
		}
	}
}
