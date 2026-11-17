//go:build integration

package scheduler

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestExperienceErasureProjectHighwaterDefersNewProjectsUntilNextCycle(t *testing.T) {
	ns := "erasure-highwater-" + uuid.NewString()
	var base erasureFixture
	for i := 0; i < 9; i++ {
		base = newErasureFixtureNamespace(t, "ttl", ns)
	}
	cursor := experienceErasureCursor{runs: map[string]experienceErasurePosition{}}
	page, err := cursor.projects(context.Background(), base.conn, ns)
	if err != nil || len(page) != 8 {
		t.Fatal(page, err)
	}
	cursor.project = page[7]
	next := base
	next.project = "ffffffff-ffff-4fff-8fff-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	next.ref.VersionID = uuid.NewString()
	next.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'new','unused','main','test','git')`, next.project, ns)
	next.seedRoot(t, "ttl")
	page, err = cursor.projects(context.Background(), base.conn, ns)
	if err != nil || len(page) != 1 {
		t.Fatalf("new projects extended current traversal and can starve revisit: %v %v", page, err)
	}
}
