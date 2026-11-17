//go:build integration

package scheduler

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestExperienceErasureActualConsumerNeedsPositiveStopNotCancelAck(t *testing.T) {
	for _, mode := range []string{"archive", "remote404", "child-unknown", "active"} {
		t.Run(mode, func(t *testing.T) {
			b := erasureBackendHTTP(t, "404")
			f := newErasureFixtureNamespace(t, "ttl", b.Namespace())
			f.usage(t, f.request, true)
			directory := f.runCopies(t)
			if mode != "remote404" {
				f.stopReceipt(t, f.run)
			}
			if mode == "child-unknown" {
				_ = f.continuation(t, false)
			}
			if mode == "active" {
				f.exec(t, `INSERT INTO pangu.node_attempts(run_id,node_run_id,node_id,attempt) VALUES($1,$1,'answer',1)`, f.run)
			}
			phase, err := b.experienceEraseRun(context.Background(), experienceErasureBinding{f.ns, f.project, f.request, f.run})
			var cancelled, purged bool
			var snapshot string
			readErr := f.conn.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pangu.run_cancellations c WHERE c.run_id=s.run_id),s.purged_at IS NOT NULL,s.snapshot::text FROM pangu.submissions s WHERE id=$1`, f.request).Scan(&cancelled, &purged, &snapshot)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "archive" {
				if err != nil || phase != "Purged" || !purged || !cancelled || strings.Contains(snapshot, "private") {
					t.Fatalf("actual consumer not erased: %s %v %v %v %s", phase, err, cancelled, purged, snapshot)
				}
				if _, statErr := os.Stat(directory); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatal("disk copy survived", statErr)
				}
			} else {
				if err == nil || purged || !cancelled {
					t.Fatalf("unknown stop treated as deletion authority: %s %v %v %v", phase, err, cancelled, purged)
				}
				if _, statErr := os.Stat(directory); statErr != nil {
					t.Fatal("unknown stop deleted disk", statErr)
				}
			}
		})
	}
}
