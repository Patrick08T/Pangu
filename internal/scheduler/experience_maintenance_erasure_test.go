//go:build integration

package scheduler

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestExperienceMaintenanceRetentionErasesActualConsumer(t *testing.T) {
	for _, rootBody := range []bool{false, true} {
		name := "already_erased_root"
		if rootBody {
			name = "root_erased_this_round"
		}
		t.Run(name, func(t *testing.T) {
			entryDB(t)
			b := erasureBackendHTTP(t, "404")
			f := newErasureFixtureNamespace(t, "ttl", b.Namespace())
			f.usage(t, f.request, true)
			directory := f.runCopies(t)
			f.stopReceipt(t, f.run)
			if rootBody {
				f.exec(t, `INSERT INTO pangu.experience_bodies(version_id,content) VALUES($1,'{"private":"expired-root"}')`, f.ref.VersionID)
			}
			if _, err := b.Retention(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			before := maintenanceConsumerState(t, f)
			if before.purged || before.cancelled || before.reports != 1 || before.rootBody != rootBody {
				t.Fatalf("preview changed actual consumer: %+v", before)
			}
			if _, err := os.Stat(directory); err != nil {
				t.Fatal("preview removed consumer directory", err)
			}
			if _, err := b.Retention(context.Background(), false); err != nil {
				t.Fatal(err)
			}
			after := maintenanceConsumerState(t, f)
			if !after.purged || !after.cancelled || after.reports != 0 || after.rootBody {
				t.Fatalf("real retention did not erase actual consumer: %+v", after)
			}
			if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("derived directory survived real maintenance", err)
			}
		})
	}
}

func TestExperienceMaintenanceRetentionPreservesConsumerWithoutStopProof(t *testing.T) {
	entryDB(t)
	b := erasureBackendHTTP(t, "404")
	f := newErasureFixtureNamespace(t, "ttl", b.Namespace())
	f.usage(t, f.request, true)
	directory := f.runCopies(t)
	if _, err := b.Retention(context.Background(), false); err == nil {
		t.Fatal("unknown stop was reported as successful erasure")
	}
	state := maintenanceConsumerState(t, f)
	if state.purged || !state.cancelled || state.reports != 1 {
		t.Fatalf("cancel receipt became stop proof: %+v", state)
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatal("unknown stop erased directory", err)
	}
}

type maintenanceConsumerSnapshot struct {
	purged, cancelled, rootBody bool
	reports                     int
}

func maintenanceConsumerState(t *testing.T, f erasureFixture) maintenanceConsumerSnapshot {
	t.Helper()
	var result maintenanceConsumerSnapshot
	err := f.conn.QueryRow(context.Background(), `SELECT s.purged_at IS NOT NULL,
 EXISTS(SELECT 1 FROM pangu.run_cancellations c WHERE c.run_id=s.run_id),
 EXISTS(SELECT 1 FROM pangu.experience_bodies b WHERE b.version_id=$2),
 (SELECT count(*) FROM pangu.research_reports r WHERE r.run_id=s.run_id AND r.body<>'')
 FROM pangu.submissions s WHERE s.id=$1`, f.request, f.ref.VersionID).Scan(&result.purged, &result.cancelled, &result.rootBody, &result.reports)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
