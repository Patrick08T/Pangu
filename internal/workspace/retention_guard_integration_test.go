//go:build integration

package workspace

import (
	"context"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestRetentionGuardAllowsRuntimeHoldingCaseSessionLock(t *testing.T) {
	tx, request, _ := experienceRetentionTx(t)
	ctx := context.Background()
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	holder, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close(ctx)
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended('evolution-case:'||$1,0))`, request); err != nil {
		t.Fatal(err)
	}
	writer, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close(ctx)
	for _, query := range []string{`UPDATE pangu.evolution_case_runtime SET outputs=outputs WHERE request_id=$1`, `UPDATE pangu.evolution_node_runs SET output=output WHERE request_id=$1`} {
		bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err = writer.Exec(bounded, query, request)
		cancel()
		if err != nil {
			t.Fatalf("runtime output write self-deadlocked behind its own case lease: %v", err)
		}
	}
}

func TestRetentionGuardRejectsRuntimeIdentityTransplant(t *testing.T) {
	for _, kind := range []string{"runtime_run", "node_key"} {
		t.Run(kind, func(t *testing.T) {
			tx, request, _ := experienceRetentionTx(t)
			query := `UPDATE pangu.evolution_case_runtime SET run_id=$2 WHERE request_id=$1`
			if kind == "node_key" {
				query = `UPDATE pangu.evolution_node_runs SET node_key=$2 WHERE request_id=$1`
			}
			if _, err := tx.Exec(context.Background(), query, request, uuid.NewString()); err == nil {
				t.Fatal("immutable runtime identity was transplanted", kind)
			}
		})
	}
}
