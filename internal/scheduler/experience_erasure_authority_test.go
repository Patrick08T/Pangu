//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type erasureFixture struct {
	conn                      *pgx.Conn
	ns, project, request, run string
	ref                       experience.Reference
}

func newErasureFixture(t *testing.T, cause string) erasureFixture {
	t.Helper()
	return newErasureFixtureNamespace(t, cause, "erasure-"+uuid.NewString())
}

func newErasureFixtureNamespace(t *testing.T, cause, namespace string) erasureFixture {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("requires independent test database")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	f := erasureFixture{conn: conn, ns: namespace, project: uuid.NewString(), request: uuid.NewString(), run: uuid.NewString()}
	f.ref = experience.Reference{VersionID: uuid.NewString(), BodyHash: strings.Repeat("a", 64), DeliveryRevision: 1}
	f.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'erasure','unused','main','test','git')`, f.project, f.ns)
	f.exec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id) VALUES($1::uuid,$2,$3,'submit',$1::text,$1::text,'{"purpose":"business","workflow":{"input":"private-accepted-input"}}',$4)`, f.request, f.ns, f.project, f.run)
	f.seedRoot(t, cause)
	return f
}

func (f erasureFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.conn.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f erasureFixture) seedRoot(t *testing.T, cause string) {
	t.Helper()
	source := experience.Source{ID: uuid.NewString(), RunID: uuid.NewString(), NodeID: "research", ProjectID: f.project, Kind: "report", Hash: strings.Repeat("b", 64)}
	raw, _ := json.Marshal([]experience.Source{source})
	seconds := 3600
	if cause == "ttl" {
		seconds = -1
	}
	f.exec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by) VALUES($1,$2,$3,$1,1,$4,$5,clock_timestamp()+make_interval(secs=>$6),'trusted')`, f.ref.VersionID, f.ns, f.project, f.ref.BodyHash, raw, seconds)
	f.exec(t, `INSERT INTO pangu.experience_states(version_id,validity,tombstoned,invalid_reason) VALUES($1,'revoked',true,'other-prior-reason')`, f.ref.VersionID)
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,1,'serving','{}','trusted')`, f.ref.VersionID)
	if cause == "withdrawn" || cause == "source_expired" || cause == "other_hash" {
		actor := uuid.NewString()
		f.exec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id) VALUES($1::uuid,$2,'test','test',$1::text)`, actor, f.ns)
		hash := source.Hash
		if cause == "other_hash" {
			hash = strings.Repeat("c", 64)
		}
		f.exec(t, `INSERT INTO pangu.learning_source_consents(id,namespace,project_id,source_id,source_kind,source_hash,run_id,node_id,actor_id,expires_at,revoked_at) VALUES(gen_random_uuid(),$1,$2,$3,'report',$4,$5,'research',$6,clock_timestamp()+make_interval(secs=>$7),CASE WHEN $8 THEN now() END)`, f.ns, f.project, source.ID, hash, source.RunID, actor, map[bool]int{true: -1, false: 3600}[cause == "source_expired"], cause != "source_expired")
	}
}

func TestExperienceErasureAuthorityRequiresPositiveImmutableCause(t *testing.T) {
	for _, cause := range []string{"ttl", "withdrawn", "source_expired", "generic", "other_hash"} {
		t.Run(cause, func(t *testing.T) {
			f := newErasureFixture(t, cause)
			tx, err := f.conn.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			got, err := experienceErasureRootTx(context.Background(), tx, f.ns, f.project, f.ref)
			want := cause == "ttl" || cause == "withdrawn" || cause == "source_expired"
			if err != nil || got != want {
				t.Fatalf("authority=%v want=%v err=%v", got, want, err)
			}
		})
	}
}

func TestExperienceErasureAuthorityRequiresCompletedFormalErasure(t *testing.T) {
	for _, mutation := range []string{"body", "active", "untombstoned", "foreign", "hash", "revision"} {
		t.Run(mutation, func(t *testing.T) {
			f := newErasureFixture(t, "ttl")
			ref := f.ref
			project := f.project
			switch mutation {
			case "body":
				f.exec(t, `INSERT INTO pangu.experience_bodies(version_id,content) VALUES($1,'{"private":"retained"}')`, f.ref.VersionID)
			case "active":
				f.exec(t, `UPDATE pangu.experience_states SET validity='active' WHERE version_id=$1`, f.ref.VersionID)
			case "untombstoned":
				f.exec(t, `UPDATE pangu.experience_states SET tombstoned=false WHERE version_id=$1`, f.ref.VersionID)
			case "foreign":
				project = uuid.NewString()
			case "hash":
				ref.BodyHash = strings.Repeat("f", 64)
			case "revision":
				ref.DeliveryRevision = 99
			}
			tx, err := f.conn.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			got, _ := experienceErasureRootTx(context.Background(), tx, f.ns, project, ref)
			if got {
				t.Fatal("incomplete, wrong-scope or forged proof authorized physical erasure")
			}
		})
	}
}
