package factory

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func Key(id string) string { return "factory:" + id }

func Prepare(ctx context.Context, ns, id string, in Input, d workflow.Definition, digest string) (*submission.Record, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = authorize(ctx, tx, ns, d.ProjectID, "run"); err != nil {
		return nil, err
	}
	if actor := identity.Current(ctx); actor.ID != "" {
		if err = identity.Authorize(ctx, actor, d.ProjectID, "configure", ""); err != nil {
			return nil, err
		}
	}
	s, err := scanSession(tx.QueryRow(ctx, "SELECT "+sessionColumns+" FROM pangu.factory_sessions WHERE namespace=$1 AND id=$2 FOR UPDATE", ns, id))
	if err != nil {
		return nil, err
	}
	if err = authorizeSession(ctx, s); err != nil {
		return nil, err
	}
	previous, err := scanTurn(tx.QueryRow(ctx, "SELECT "+turnColumns+" FROM pangu.factory_turns WHERE session_id=$1 AND id=$2", id, in.ID))
	if err == nil {
		if previous.Input != in {
			return nil, catalog.ErrConflict
		}
		return submission.Get(ctx, ns, previous.RequestID)
	}
	if !errors.Is(err, catalog.ErrNotFound) {
		return nil, err
	}
	if s.Revision != in.Revision || s.ProjectID != d.ProjectID {
		return nil, catalog.ErrConflict
	}
	if err = canAppend(ctx, tx, id); err != nil {
		return nil, err
	}
	record, err := submission.PrepareTx(ctx, tx, ns, "submit", Key(in.ID), submission.Snapshot{Workflow: d})
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO pangu.factory_turns(id,session_id,sequence,request_id,input,skill_digest,status) VALUES($1,$2,$3,$4,$5,$6,'pending')", in.ID, id, s.Revision, record.ID, data, digest)
	if err != nil {
		return nil, err
	}
	title := []rune(in.Message)
	if len(title) > 40 {
		title = title[:40]
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.factory_sessions SET revision=revision+1,title=CASE WHEN revision=1 THEN $2 ELSE title END,updated_at=now() WHERE id=$1", id, string(title))
	if err != nil {
		return nil, err
	}
	return record, tx.Commit(ctx)
}

func canAppend(ctx context.Context, tx pgx.Tx, id string) error {
	var count int
	var pending bool
	if err := tx.QueryRow(ctx, "SELECT count(*),coalesce(bool_or(status='pending'),false) FROM pangu.factory_turns WHERE session_id=$1", id).Scan(&count, &pending); err != nil {
		return err
	}
	if pending {
		return catalog.ErrConflict
	}
	if count >= 20 {
		return invalid("本次对话已达 20 轮，请新建对话")
	}
	return nil
}

func Complete(ctx context.Context, ns, id, turn, raw, failure string) error {
	s, err := Get(ctx, ns, id)
	if err != nil {
		return err
	}
	var current *Turn
	for i := range s.Turns {
		if s.Turns[i].ID == turn {
			current = &s.Turns[i]
		}
	}
	if current == nil {
		return catalog.ErrNotFound
	}
	if current.Status != "pending" {
		return nil
	}
	var reply *Reply
	if failure == "" {
		reply, err = Parse(raw)
		if err == nil && reply.Proposal != nil {
			err = checkTargets(*reply.Proposal, current.Input)
			if err == nil {
				err = bindModels(ctx, ns, reply.Proposal)
			}
			if err == nil {
				err = checkOptimization(ctx, ns, current.Input, reply.Proposal)
			}
			if err == nil {
				err = catalog.CheckBundle(ctx, ns, *reply.Proposal)
			}
		}
		if err != nil {
			failure = err.Error()
			reply = nil
		}
	}
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
	if _, err = tx.Exec(ctx, "SELECT id FROM pangu.factory_sessions WHERE namespace=$1 AND id=$2 FOR UPDATE", ns, id); err != nil {
		return err
	}
	status := "ready"
	if failure != "" {
		status = "failed"
	}
	data, err := json.Marshal(reply)
	if err != nil {
		return err
	}
	updated, err := tx.Exec(ctx, "UPDATE pangu.factory_turns SET status=$1,reply=$2,error=$3,finished_at=now() WHERE session_id=$4 AND id=$5 AND status='pending'", status, data, failure, id, turn)
	if err != nil {
		return err
	}
	if updated.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, "UPDATE pangu.factory_sessions SET revision=revision+1,updated_at=now() WHERE id=$1", id)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func Apply(ctx context.Context, ns, id, turn string, revision int) (*catalog.BundleResult, error) {
	s, err := Get(ctx, ns, id)
	if err != nil {
		return nil, err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = authorize(ctx, tx, ns, "", "manage_catalog"); err != nil {
		return nil, err
	}
	s, err = scanSession(tx.QueryRow(ctx, "SELECT "+sessionColumns+" FROM pangu.factory_sessions WHERE namespace=$1 AND id=$2 FOR UPDATE", ns, id))
	if err != nil {
		return nil, err
	}
	if err = authorizeSession(ctx, s); err != nil {
		return nil, err
	}
	t, err := scanTurn(tx.QueryRow(ctx, "SELECT "+turnColumns+" FROM pangu.factory_turns WHERE session_id=$1 AND id=$2", id, turn))
	if err != nil {
		return nil, err
	}
	if t.Result != nil {
		return t.Result, nil
	}
	if s.Revision != revision || t.Status != "ready" || t.Reply == nil || t.Reply.Proposal == nil {
		return nil, catalog.ErrConflict
	}
	var latest string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM pangu.factory_turns WHERE session_id=$1 ORDER BY sequence DESC LIMIT 1", id).Scan(&latest); err != nil {
		return nil, err
	}
	if latest != turn {
		return nil, catalog.ErrConflict
	}
	if err = checkTargets(*t.Reply.Proposal, t.Input); err != nil {
		return nil, err
	}
	out, err := catalog.ApplyBundleTx(ctx, tx, ns, *t.Reply.Proposal)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.factory_turns SET status='applied',result=$1 WHERE id=$2", data, turn)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "UPDATE pangu.factory_sessions SET revision=revision+1,updated_at=now() WHERE id=$1", id)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
