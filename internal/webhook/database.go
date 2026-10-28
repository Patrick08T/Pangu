package webhook

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	ProjectID  string `json:"project_id"`
	VersionID  string `json:"version_id"`
	Repository string `json:"repository"`
	Label      string `json:"label"`
	SecretRef  string `json:"secret_ref"`
	Enabled    bool   `json:"enabled"`
}
type Hook struct {
	ID      string `json:"id"`
	ActorID string `json:"actor_id"`
	Config
	CreatedAt time.Time `json:"created_at"`
}

const columns = `id::text,actor_id::text,project_id::text,version_id::text,repository,label,secret_ref,enabled,created_at`

var repository = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var secret = regexp.MustCompile(`^PANGU_HOOK_SECRET_[A-Z][A-Z0-9_]{0,63}$`)

func (c Config) Validate() error {
	for _, id := range []string{c.ProjectID, c.VersionID} {
		if _, err := uuid.Parse(id); err != nil {
			return ErrInvalid
		}
	}
	if !repository.MatchString(c.Repository) || len(c.Repository) > 256 || strings.TrimSpace(c.Label) == "" || len(c.Label) > 64 || !secret.MatchString(c.SecretRef) || !c.Enabled {
		return ErrInvalid
	}
	return nil
}

func scan(row pgx.Row) (*Hook, error) {
	item := &Hook{}
	err := row.Scan(&item.ID, &item.ActorID, &item.ProjectID, &item.VersionID, &item.Repository, &item.Label, &item.SecretRef, &item.Enabled, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return item, err
}

func Create(ctx context.Context, namespace string, c Config) (*Hook, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	version, err := catalog.ReadVersion(ctx, namespace, c.VersionID)
	if err != nil {
		return nil, err
	}
	if version.Archived || (version.Kind != "agent" && version.Kind != "workflow") {
		return nil, ErrInvalid
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scan(conn.QueryRow(ctx, `INSERT INTO pangu.github_hooks(id,namespace,actor_id,project_id,version_id,repository,label,secret_ref) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+columns, uuid.NewString(), namespace, identity.Current(ctx).ID, c.ProjectID, c.VersionID, c.Repository, c.Label, c.SecretRef))
}

func Get(ctx context.Context, namespace, id string) (*Hook, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return scan(conn.QueryRow(ctx, "SELECT "+columns+" FROM pangu.github_hooks WHERE namespace=$1 AND id=$2", namespace, id))
}

func List(ctx context.Context, namespace, project string, offset int) ([]Hook, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT "+columns+" FROM pangu.github_hooks WHERE namespace=$1 AND project_id=$2 ORDER BY created_at DESC,id DESC LIMIT 100 OFFSET $3", namespace, project, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Hook{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func Disable(ctx context.Context, namespace, id string) error {
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
	if err = identity.Lock(ctx, tx, namespace); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.github_hooks SET enabled=false WHERE namespace=$1 AND id=$2", namespace, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
