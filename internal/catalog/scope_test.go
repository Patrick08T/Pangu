package catalog

import (
	"errors"
	"testing"
)

func TestVersionScopeRejectsPrivateAndUnstableDefaults(t *testing.T) {
	cases := []struct {
		name, source, project, state string
		scope                        VersionScope
		allowed                      bool
	}{
		{name: "legacy shared", allowed: true},
		{name: "manual any project", source: "manual", scope: VersionScope{ProjectID: "other"}, allowed: true},
		{name: "missing project", source: "auto", project: "owner", state: "stable"},
		{name: "foreign project", source: "auto", project: "owner", state: "stable", scope: VersionScope{ProjectID: "other"}},
		{name: "owner stable", source: "auto", project: "owner", state: "stable", scope: VersionScope{ProjectID: "owner"}, allowed: true},
		{name: "observing default", source: "auto", project: "owner", state: "observing", scope: VersionScope{ProjectID: "owner"}},
		{name: "observing authorized", source: "auto", project: "owner", state: "observing", scope: VersionScope{ProjectID: "owner", AllowObserving: true}, allowed: true},
		{name: "revoked trial", source: "auto", project: "owner", state: "revoked", scope: VersionScope{ProjectID: "owner", AllowObserving: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Version{SourceKind: tc.source, ProjectID: tc.project, State: tc.state}
			err := checkVersionScope(v, tc.scope)
			if (err == nil) != tc.allowed {
				t.Fatalf("scope错误: %v", err)
			}
			if err != nil && !errors.Is(err, ErrNotFound) {
				t.Fatalf("无权读取泄露版本存在: %v", err)
			}
		})
	}
}

func TestVersionScopeRejectsClearedBodyEvenForTrial(t *testing.T) {
	v := Version{SourceKind: "auto", ProjectID: "owner", State: "stable", Tombstoned: true}
	if err := checkVersionScope(v, VersionScope{ProjectID: "owner", AllowObserving: true}); err == nil {
		t.Fatal("已清理正文仍可读取或运行")
	}
}
