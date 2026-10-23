package database

import (
	"context"
	"strings"
	"testing"
)

func TestDatabaseConfigurationIsReadFreshAndRedacted(t *testing.T) {
	for _, value := range []string{"", "postgres://user:private-secret@%zz/db", "postgres://user:private-secret@localhost/hatchet"} {
		t.Setenv("PANGU_DATABASE_URL", value)
		err := Check(context.Background())
		if err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("unexpected result: %v", err)
		}
	}
}

func TestMigrationHistoryValidation(t *testing.T) {
	files := []migration{{name: "0001_namespace.sql", sql: "CREATE SCHEMA pangu;"}}
	checksum := files[0].checksum()
	for _, tc := range []struct {
		name    string
		history map[string]string
		ready   bool
	}{
		{"missing", map[string]string{}, false},
		{"matched", map[string]string{files[0].name: checksum}, true},
		{"changed", map[string]string{files[0].name: "changed"}, false},
		{"future", map[string]string{files[0].name: checksum, "9999_future.sql": "value"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHistory(files, tc.history, true)
			if (err == nil) != tc.ready {
				t.Fatalf("ready=%v err=%v", tc.ready, err)
			}
		})
	}
}
