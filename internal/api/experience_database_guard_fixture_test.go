//go:build integration

package api

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Parse only: rejected targets never reach a connection attempt or fixture seed.
func parseExperienceTestDatabase(value string) (*pgx.ConnConfig, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("experience API integration tests require PANGU_TEST_DATABASE_URL")
	}
	config, err := pgx.ParseConfig(value)
	if err != nil {
		return nil, errors.New("experience API test database configuration is invalid")
	}
	if config.Host != "127.0.0.1" || config.Port != 55439 || config.Database != "pangu_test" || config.User != "pangu_test_runtime" {
		return nil, errors.New("experience API tests require the isolated pangu_test database and runtime role at 127.0.0.1:55439")
	}
	for _, fallback := range config.Fallbacks {
		if fallback == nil || fallback.Host != "127.0.0.1" || fallback.Port != 55439 {
			return nil, errors.New("experience API test database fallback is outside the isolated target")
		}
	}
	return config, nil
}
