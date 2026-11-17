//go:build integration

package experience

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestFrozenSelectionReadRejectsCorruptPersistedProof(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Usage)
	}{
		{"watermark", func(u *Usage) { u.WatermarkHash = strings.Repeat("b", 64) }},
		{"injection", func(u *Usage) { u.InjectionHash = strings.Repeat("b", 64) }},
		{"request", func(u *Usage) { u.Input.RunID = uuid.NewString() }},
		{"project", func(u *Usage) { u.Input.ProjectID = uuid.NewString() }},
		{"bound_input", func(u *Usage) { u.Input.BoundInputHash = strings.Repeat("b", 64) }},
		{"generation", func(u *Usage) { u.Input.Generation++ }},
		{"watermark_generation", func(u *Usage) { u.Input.WatermarkGeneration++ }},
		{"role", func(u *Usage) { u.Input.RoleID = "other" }},
		{"role_version", func(u *Usage) { u.Input.RoleVersion = uuid.NewString() }},
		{"cohort", func(u *Usage) { u.Input.CanaryScope = "other" }},
		{"membership", func(u *Usage) { u.References[0].DeliveryRevision++; u.InjectionHash = referenceHash(u.References) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, input, usage := frozenReadFixture(t)
			input.NodeID = "forged"
			usage.Input = input
			test.change(&usage)
			insertFrozenUsage(t, f, input, usage)
			if _, _, _, err := readFrozen(t, f, input); err == nil {
				t.Fatal("corrupt frozen proof was accepted")
			}
		})
	}
}

func insertFrozenUsage(t *testing.T, f storeFixture, input SelectionInput, usage Usage) {
	t.Helper()
	raw, err := json.Marshal(usage)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(f.ctx)
	if _, err = conn.Exec(f.ctx, `INSERT INTO pangu.experience_usage(namespace,project_id,identity_hash,usage) VALUES($1,$2,$3,$4)`, f.scope.Namespace, f.scope.ProjectID, selectionIdentity(input), raw); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenSelectionReadPreservesInfrastructureErrors(t *testing.T) {
	f, input, _ := frozenReadFixture(t)
	sentinel := errors.New("source service unavailable")
	f.store.VerifySource = func(context.Context, pgx.Tx, Scope, Source) error { return sentinel }
	if _, _, _, err := readFrozen(t, f, input); !errors.Is(err, sentinel) {
		t.Fatal("unknown source error replaced", err)
	}
	f.store.VerifySource = func(ctx context.Context, tx pgx.Tx, _ Scope, _ Source) error {
		_, err := tx.Exec(ctx, `SELECT nonexistent_frozen_selection_column FROM pangu.experience_usage`)
		return err
	}
	_, _, _, err := readFrozen(t, f, input)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "42703" {
		t.Fatal("SQL error replaced by proof denial", err)
	}
}

func TestFrozenSelectionReadUsesCurrentAuthorization(t *testing.T) {
	f, input, _ := frozenReadFixture(t)
	f.scope.ActorID = uuid.NewString()
	if _, _, _, err := readFrozen(t, f, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("frozen proof bypassed current actor", err)
	}
}
