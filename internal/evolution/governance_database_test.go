//go:build integration

package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type governanceFixture struct {
	scope  Scope
	policy experience.Policy
}

func newGovernanceFixture(t *testing.T) governanceFixture {
	t.Helper()
	url := os.Getenv("PANGU_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("integration requires PANGU_TEST_DATABASE_URL; not skipped")
	}
	t.Setenv("PANGU_DATABASE_URL", url)
	f := governanceFixture{scope: Scope{Namespace: uuid.NewString(), ProjectID: uuid.NewString(), ActorID: uuid.NewString()}, policy: governancePolicy()}
	f.policy.ProjectID = f.scope.ProjectID
	f.policy.ActorID = f.scope.ActorID
	f.policy.Revision = 1
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(context.Background(), `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'evolution','unused','main','test','git')`, f.scope.ProjectID, f.scope.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(context.Background(), `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1,$2,'test','test',$3,true)`, f.scope.ActorID, f.scope.Namespace, f.scope.ActorID)
	if err != nil {
		t.Fatal(err)
	}
	f.savePolicy(t)
	return f
}

func (f governanceFixture) savePolicy(t *testing.T) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	data, _ := json.Marshal(f.policy)
	_, err = conn.Exec(context.Background(), `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,$3,$4,$5)`, f.scope.Namespace, f.scope.ProjectID, f.policy.Revision, f.scope.ActorID, data)
	if err != nil {
		t.Fatal(err)
	}
}

func governanceTx(f governanceFixture, fn func(context.Context, pgx.Tx) error) error {
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestBudgetLedgerSerializesAndSurvivesPolicyRevisions(t *testing.T) {
	f := newGovernanceFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- governanceTx(f, func(ctx context.Context, tx pgx.Tx) error {
				return ReserveBudgetTx(ctx, tx, f.scope, 1, "evaluation", uuid.NewString(), experience.Budget{Calls: 6, Tokens: 600})
			})
		}()
	}
	wg.Wait()
	close(results)
	passed, blocked := 0, 0
	for err := range results {
		if err == nil {
			passed++
		} else if errors.Is(err, ErrBudget) {
			blocked++
		} else {
			t.Fatal(err)
		}
	}
	if passed != 1 || blocked != 1 {
		t.Fatalf("并发预算: passed=%d blocked=%d", passed, blocked)
	}
	f.policy.Revision = 2
	f.savePolicy(t)
	err := governanceTx(f, func(ctx context.Context, tx pgx.Tx) error {
		return ReserveBudgetTx(ctx, tx, f.scope, 2, "evaluation", "second-policy", experience.Budget{Calls: 5, Tokens: 500})
	})
	if !errors.Is(err, ErrBudget) {
		t.Fatal("策略修订重置了预算", err)
	}
}

func TestBudgetReplayAndRevocationAreAuthoritative(t *testing.T) {
	f := newGovernanceFixture(t)
	reserve := func() error {
		return governanceTx(f, func(ctx context.Context, tx pgx.Tx) error {
			return ReserveBudgetTx(ctx, tx, f.scope, f.policy.Revision, "evaluation", "same", experience.Budget{Calls: 10, Tokens: 1000})
		})
	}
	if err := reserve(); err != nil {
		t.Fatal(err)
	}
	if err := reserve(); err != nil {
		t.Fatal("幂等重试重复计费", err)
	}
	f.policy.Revision++
	f.policy.Revoked = true
	f.savePolicy(t)
	if err := reserve(); !errors.Is(err, ErrForbidden) {
		t.Fatal("撤权后旧预留仍生效", err)
	}
}
