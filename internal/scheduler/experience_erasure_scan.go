package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type experienceErasurePosition struct{ after, upper string }
type experienceErasureCursor struct {
	sync.Mutex
	project, upper string
	runs           map[string]experienceErasurePosition
}

var experienceErasureCursors sync.Map

const experienceErasureCandidateSQL = `s.namespace=$1 AND s.project_id=$2 AND s.purged_at IS NULL
 AND coalesce(s.snapshot->>'purpose','') IN ('','business') AND (
 EXISTS(SELECT 1 FROM pangu.experience_usage u WHERE u.namespace=s.namespace AND u.project_id=s.project_id AND u.usage#>>'{input,run_id}'=s.id::text)
 OR EXISTS(SELECT 1 FROM pangu.experience_artifacts a WHERE a.namespace=s.namespace AND a.project_id=s.project_id AND a.artifact->>'run_id'=s.run_id::text)
 OR s.snapshot#>'{fork,reused}' IS NOT NULL)`

const experienceErasureProjectSQL = `p.namespace=$1
 AND EXISTS(SELECT 1 FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id
 WHERE v.namespace=p.namespace AND v.project_id=p.id AND s.tombstoned AND s.validity IN ('revoked','expired')
 AND NOT EXISTS(SELECT 1 FROM pangu.experience_bodies b WHERE b.version_id=v.id))`

func (c *experienceErasureCursor) projects(ctx context.Context, conn *pgx.Conn, namespace string) ([]string, error) {
	if c.upper == "" {
		err := conn.QueryRow(ctx, `SELECT p.id::text FROM pangu.projects p WHERE `+experienceErasureProjectSQL+` ORDER BY p.id DESC LIMIT 1`, namespace).Scan(&c.upper)
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	rows, err := conn.Query(ctx, `SELECT p.id::text FROM pangu.projects p WHERE `+experienceErasureProjectSQL+` AND p.id::text>$2 AND p.id::text<=$3 ORDER BY p.id LIMIT 8`, namespace, c.project, c.upper)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (c *experienceErasureCursor) page(ctx context.Context, conn *pgx.Conn, namespace, project string) ([]experienceErasureBinding, error) {
	pos := c.runs[project]
	if pos.upper == "" {
		err := conn.QueryRow(ctx, `SELECT s.id::text FROM pangu.submissions s WHERE `+experienceErasureCandidateSQL+` ORDER BY s.id DESC LIMIT 1`, namespace, project).Scan(&pos.upper)
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	c.runs[project] = pos
	rows, err := conn.Query(ctx, `SELECT s.id::text,coalesce(s.run_id::text,'') FROM pangu.submissions s WHERE `+experienceErasureCandidateSQL+` AND s.id::text>$3 AND s.id::text<=$4 ORDER BY s.id LIMIT 8`, namespace, project, pos.after, pos.upper)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (experienceErasureBinding, error) {
		item := experienceErasureBinding{namespace: namespace, project: project}
		err := row.Scan(&item.request, &item.run)
		return item, err
	})
}

func (c *experienceErasureCursor) pass(ctx context.Context, b Backend) error {
	conn, err := database.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	projects, err := c.projects(ctx, conn.Conn(), b.Namespace())
	if err != nil {
		return err
	}
	var failures []error
	counts := map[string]int{}
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		c.project = project
		check, cancel := context.WithTimeout(ctx, 2*time.Second)
		items, pageErr := c.page(check, conn.Conn(), b.Namespace(), project)
		cancel()
		if pageErr != nil {
			failures = append(failures, pageErr)
			continue
		}
		for _, item := range items {
			if err = ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			pos := c.runs[project]
			pos.after = item.request
			c.runs[project] = pos
			check, cancel := context.WithTimeout(ctx, 2*time.Second)
			phase, itemErr := b.experienceEraseRun(check, item)
			cancel()
			if phase != "" {
				counts[phase]++
			}
			if itemErr != nil {
				failures = append(failures, itemErr)
			}
		}
		if len(items) < 8 || c.runs[project].after == c.runs[project].upper {
			delete(c.runs, project)
		}
	}
	if len(projects) < 8 || c.project == c.upper {
		c.project, c.upper = "", ""
	}
	if len(counts) > 0 {
		slog.Info("experience derived-copy erasure pass", "namespace", b.Namespace(), "phases", counts)
	}
	return errors.Join(failures...)
}
