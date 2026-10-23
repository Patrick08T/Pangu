package diagnostic

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRedactSecretsBeforeTruncation(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "the-real-test-secret")
	input := "Bearer abcdefghijkl token=hidden-value https://user:password@example.org the-real-test-secret"
	got, truncated := Text(input, 4096)
	for _, secret := range []string{"abcdefghijkl", "hidden-value", "password", "the-real-test-secret"} {
		if strings.Contains(got, secret) {
			t.Errorf("secret leaked: %s", got)
		}
	}
	if truncated {
		t.Fatal("short message truncated")
	}
	got, truncated = Text(strings.Repeat("中", 10), 7)
	if !truncated || len(got) > 7 || !utf8.ValidString(got) {
		t.Fatalf("invalid bounded UTF-8: %q %v", got, truncated)
	}
}

func TestQuotedCredentialsAreRedactedWithoutEnvironment(t *testing.T) {
	for _, value := range []string{`{"token":"json-secret-value"}`, `{"api_key":"json-secret-value"}`,
		`password="two word secret"`, `secret='two word secret'`, `{"password":"escaped\\\"secret"}`} {
		got, _ := Text(value, 4096)
		if strings.Contains(got, "json-secret-value") || strings.Contains(got, " word ") || strings.Contains(got, "escaped") {
			t.Errorf("泄漏凭据 %s → %s", value, got)
		}
	}
}
