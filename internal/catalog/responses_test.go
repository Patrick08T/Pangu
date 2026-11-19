package catalog

import "testing"

func TestResponsesConnectionRequiresExplicitValidReasoning(t *testing.T) {
	connection := Connection{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-6.2-sol", SecretRef: "PANGU_SECRET_OPENAI", API: "responses", ReasoningEffort: "high"}
	if err := connection.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []Connection{
		{Provider: "codex", API: "responses", ReasoningEffort: "high"},
		{Provider: "openai", BaseURL: connection.BaseURL, Model: connection.Model, SecretRef: connection.SecretRef, ReasoningEffort: "high"},
		{Provider: "openai", BaseURL: connection.BaseURL, Model: connection.Model, SecretRef: connection.SecretRef, API: "responses"},
		{Provider: "openai", BaseURL: connection.BaseURL, Model: connection.Model, SecretRef: connection.SecretRef, API: "responses", ReasoningEffort: "fast"},
	} {
		if err := value.Validate(); err == nil {
			t.Errorf("错误接受配置: %+v", value)
		}
	}
}
