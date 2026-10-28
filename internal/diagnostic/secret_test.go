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
		`password="two word secret"`, `secret='two word secret'`, `{"password":"escaped\\\"secret"}`,
		`{"arguments":"{\"password\":\"two word secret\",\"api_key\":\"json-secret-value\"}"}`} {
		got, _ := Text(value, 4096)
		if strings.Contains(got, "json-secret-value") || strings.Contains(got, " word ") || strings.Contains(got, "escaped") {
			t.Errorf("泄漏凭据 %s → %s", value, got)
		}
	}
}

func TestBareCredentialsAndPrivateKeysAreRedacted(t *testing.T) {
	for _, value := range []string{
		"sk-proj-abcdefghijklmnopqrstuvwxyz0123456789",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJwcml2YXRlIn0.c2lnbmF0dXJlX3ByaXZhdGU",
		"-----BEGIN RSA PRIVATE KEY-----\nprivate-key-material\n-----END RSA PRIVATE KEY-----",
		`-----BEGIN PRIVATE KEY-----\nprivate-key-material\n-----END PRIVATE KEY-----`,
		"-----BEGIN OPENSSH PRIVATE KEY-----\npartial-private-key-material",
	} {
		got, _ := Text(value, 8192)
		if got != "[已脱敏]" || !Sensitive([]byte(value)) {
			t.Errorf("裸凭据未脱敏: %q", got)
		}
	}
}
