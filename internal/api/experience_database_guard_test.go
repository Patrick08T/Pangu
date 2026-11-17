//go:build integration

package api

import (
	"strings"
	"testing"
)

func TestExperienceDatabaseGuardRejectsUncontrolledTargetsWithoutConnecting(t *testing.T) {
	const good = "postgresql://pangu_test_runtime:guard-secret-only@127.0.0.1:55439/pangu_test?sslmode=disable"
	for _, tc := range []struct {
		name, dsn string
		allowed   bool
	}{
		{"isolated_url", good, true},
		{"isolated_keyword", "host=127.0.0.1 port=55439 user=pangu_test_runtime dbname=pangu_test password=guard-secret-only sslmode=disable", true},
		{"same_endpoint_tls_fallback", strings.Replace(good, "sslmode=disable", "sslmode=prefer", 1), true},
		{"empty", "", false},
		{"whitespace", " \t\n", false},
		{"foreign_host", strings.Replace(good, "127.0.0.1", "192.0.2.1", 1), false},
		{"hostname", strings.Replace(good, "127.0.0.1", "localhost", 1), false},
		{"wrong_port", strings.Replace(good, "55439", "5432", 1), false},
		{"wrong_database", strings.Replace(good, "/pangu_test?", "/other?", 1), false},
		{"wrong_user", strings.Replace(good, "pangu_test_runtime:", "postgres:", 1), false},
		{"substring_only", strings.Replace(good, "127.0.0.1", "192.0.2.1", 1) + "&application_name=127.0.0.1:55439", false},
		{"unix_socket", "host=/tmp port=55439 user=pangu_test_runtime dbname=pangu_test sslmode=disable", false},
		{"foreign_fallback", "host=127.0.0.1,192.0.2.1 port=55439,55439 user=pangu_test_runtime dbname=pangu_test sslmode=disable", false},
		{"wrong_fallback_port", "host=127.0.0.1,127.0.0.1 port=55439,5432 user=pangu_test_runtime dbname=pangu_test sslmode=disable", false},
		{"malformed_with_password", strings.Replace(good, "55439", "bad-port", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := parseExperienceTestDatabase(tc.dsn)
			if tc.allowed {
				if err != nil || config == nil {
					t.Fatal("isolated test target was rejected")
				}
				return
			}
			if err == nil || config != nil {
				t.Fatal("uncontrolled test target was accepted")
			}
			if strings.Contains(err.Error(), "guard-secret-only") || strings.Contains(err.Error(), "postgresql://") {
				t.Fatal("database guard exposed connection credentials")
			}
		})
	}
}
