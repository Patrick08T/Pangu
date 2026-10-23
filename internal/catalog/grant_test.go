package catalog

import (
	"encoding/json"
	"testing"
)

func TestCredentialGrantBindsProjectEndpointAndModelAndReloads(t *testing.T) {
	c := Connection{BaseURL: "https://model.example/v1", Model: "test", SecretRef: "PANGU_SECRET_TEST"}
	data, _ := json.Marshal([]map[string]any{{"project_id": "project-a", "base_url": c.BaseURL, "secret_ref": c.SecretRef, "models": []string{"test"}}})
	t.Setenv("PANGU_CONNECTION_GRANTS", string(data))
	if err := c.Granted("project-a"); err != nil {
		t.Fatal(err)
	}
	if err := c.Granted("project-b"); err == nil {
		t.Fatal("跨项目借用凭据")
	}
	c.BaseURL = "https://evil.example/v1"
	if err := c.Granted("project-a"); err == nil {
		t.Fatal("凭据发送到未登记端点")
	}
	c.BaseURL = "https://model.example/v1"
	c.Model = "other"
	if err := c.Granted("project-a"); err == nil {
		t.Fatal("使用未登记模型")
	}
	c.Model = "test"
	t.Setenv("PANGU_CONNECTION_GRANTS", "[]")
	if err := c.Granted("project-a"); err == nil {
		t.Fatal("登记撤回后仍使用缓存")
	}
}
