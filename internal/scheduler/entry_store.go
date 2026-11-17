package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/jackc/pgx/v5"
)

type entryRequest struct{ namespace, project, entry, input, key string }
type entryBinding struct {
	version, stable, intent  string
	revision, policyRevision int64
	generation               int
}

// All authority reads and acceptance share the same lock order as rollout/policy writes.
func entryTransaction(ctx context.Context, r entryRequest, fn func(pgx.Tx) error) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = identity.Lock(ctx, tx, r.namespace); err != nil {
		return err
	}
	if err = experience.Lock(ctx, tx, r.namespace, r.project); err != nil {
		return err
	}
	if err = authorizeEntryActor(ctx, tx, r); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func authorizeEntryActor(ctx context.Context, tx pgx.Tx, r entryRequest) error {
	actor := identity.Current(ctx)
	if actor.ID == "" {
		return identity.ErrUnauthenticated
	}
	if actor.Namespace != r.namespace {
		return identity.ErrForbidden
	}
	var allowed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.users u JOIN pangu.projects p ON p.namespace=u.namespace
 WHERE u.id=$1 AND u.namespace=$2 AND p.id=$3 AND NOT u.disabled AND (u.is_admin OR EXISTS(
 SELECT 1 FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=p.id AND m.enabled AND m.role IN ('operator','manager'))))`, actor.ID, r.namespace, r.project).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return identity.ErrForbidden
	}
	if actor.TokenID == "" {
		return nil
	}
	if actor.ProjectID != r.project || !slices.Contains(actor.Scopes, "run") {
		return identity.ErrForbidden
	}
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.api_tokens WHERE id=$1 AND user_id=$2 AND project_id=$3
 AND revoked_at IS NULL AND expires_at>clock_timestamp() AND 'run'=ANY(scopes))`, actor.TokenID, actor.ID, r.project).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return identity.ErrForbidden
	}
	return nil
}

func entryPolicy(ctx context.Context, tx pgx.Tx, r entryRequest) (int64, error) {
	var data []byte
	var owner string
	var revision int64
	err := tx.QueryRow(ctx, `SELECT policy,actor_id::text,revision FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, r.namespace, r.project).Scan(&data, &owner, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, identity.ErrForbidden
	}
	if err != nil {
		return 0, err
	}
	var policy experience.Policy
	if err = json.Unmarshal(data, &policy); err != nil {
		return 0, err
	}
	if !policy.Enabled || !policy.AutoPublish || policy.Revoked || policy.ActorID != owner || policy.ProjectID != r.project || policy.Revision != revision || !slices.Contains(policy.Entrypoints, r.entry) {
		return 0, identity.ErrForbidden
	}
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.users u WHERE u.id=$1 AND u.namespace=$2 AND NOT u.disabled AND (u.is_admin OR EXISTS(
 SELECT 1 FROM pangu.memberships m WHERE m.user_id=u.id AND m.project_id=$3 AND m.enabled AND m.role='manager')))`, owner, r.namespace, r.project).Scan(&allowed)
	if err != nil {
		return 0, err
	}
	if !allowed {
		return 0, identity.ErrForbidden
	}
	return revision, nil
}

func readEntryBinding(ctx context.Context, tx pgx.Tx, r entryRequest) (entryBinding, error) {
	var b entryBinding
	revision, err := entryPolicy(ctx, tx, r)
	if err != nil {
		return b, err
	}
	b.policyRevision = revision
	err = tx.QueryRow(ctx, `SELECT adopted_version::text,stable_version::text,coalesce(intent_id::text,''),revision,generation
 FROM pangu.evolution_bindings WHERE namespace=$1 AND project_id=$2 AND entry_key=$3 FOR SHARE`, r.namespace, r.project, r.entry).Scan(&b.version, &b.stable, &b.intent, &b.revision, &b.generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, identity.ErrForbidden
	}
	return b, err
}

func entryDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// Match only caller-controlled identity. Never rebuild a retry payload from today's binding.
func findEntryRequest(ctx context.Context, tx pgx.Tx, r entryRequest) (*submission.Record, error) {
	item := &submission.Record{Status: "pending"}
	var body []byte
	var inputHash string
	err := tx.QueryRow(ctx, `SELECT id::text,project_id::text,operation,coalesce(run_id::text,''),created_at,snapshot,coalesce(created_by::text,''),task_id::text,input_hash
 FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND operation='entry' AND key_hash=$3`, r.namespace, r.project, entryDigest(r.key)).Scan(&item.ID, &item.ProjectID, &item.Operation, &item.RunID, &item.CreatedAt, &body, &item.CreatedBy, &item.TaskID, &inputHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, submission.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(body, &item.Snapshot); err != nil {
		return nil, err
	}
	s := item.Snapshot
	if s.Purpose != "business" || s.EntryKey != r.entry || s.EntryRevision < 1 || inputHash != entryDigest(r.input) || item.CreatedBy != identity.Current(ctx).ID {
		return nil, submission.ErrConflict
	}
	if item.RunID != "" {
		item.Status = "accepted"
	}
	return item, nil
}

func loadEntry(ctx context.Context, r entryRequest) (*submission.Record, entryBinding, error) {
	var item *submission.Record
	var binding entryBinding
	err := entryTransaction(ctx, r, func(tx pgx.Tx) error {
		var err error
		item, err = findEntryRequest(ctx, tx, r)
		if !errors.Is(err, submission.ErrNotFound) {
			return err
		}
		binding, err = readEntryBinding(ctx, tx, r)
		return err
	})
	return item, binding, err
}

func acceptEntry(ctx context.Context, r entryRequest, expected entryBinding, snapshot submission.Snapshot) (*submission.Record, error) {
	var item *submission.Record
	err := entryTransaction(ctx, r, func(tx pgx.Tx) error {
		var err error
		item, err = findEntryRequest(ctx, tx, r)
		if !errors.Is(err, submission.ErrNotFound) {
			return err
		}
		current, err := readEntryBinding(ctx, tx, r)
		if err != nil {
			return err
		}
		if current != expected {
			return submission.ErrConflict
		}
		if err = checkEntryVersions(ctx, tx, r, snapshot.Workflow); err != nil {
			return err
		}
		item, err = submission.PrepareTx(ctx, tx, r.namespace, "entry", r.key, snapshot)
		return err
	})
	return item, err
}
