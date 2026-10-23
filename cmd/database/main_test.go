package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectProvisioningUnsafeTargets(t *testing.T) {
	t.Setenv("PGPASSWORD", "")
	t.Setenv("PGPASSFILE", filepath.Join(t.TempDir(), "absent-pgpass"))
	for _, value := range []string{"", "invalid%private-secret", "postgres://postgres:private-secret@localhost/hatchet", "postgres://app@localhost/pangu"} {
		_, err := target(value)
		if err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("unsafe target or credential leak: %v", err)
		}
	}
}
