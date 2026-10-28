package execution

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/jackc/pgx/v5"
)

func Control(ctx context.Context, actor identity.Actor, run string, input DebugCommand) (*DebugState, error) {
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
	if err = identity.Lock(ctx, tx, actor.Namespace); err != nil {
		return nil, err
	}
	var project, owner string
	err = tx.QueryRow(ctx, "SELECT project_id::text,created_by::text FROM pangu.submissions WHERE namespace=$1 AND run_id=$2 AND purged_at IS NULL", actor.Namespace, run).Scan(&project, &owner)
	if err != nil {
		return nil, identity.ErrForbidden
	}
	if err = identity.Authorize(ctx, actor, project, "manage_run", owner); err != nil {
		return nil, err
	}
	if err = debugLock(ctx, tx, run); err != nil {
		return nil, err
	}
	state, err := changeDebug(ctx, tx, run, input)
	if err != nil {
		return nil, err
	}
	return state, tx.Commit(ctx)
}

func changeDebug(ctx context.Context, tx pgx.Tx, run string, input DebugCommand) (*DebugState, error) {
	if input.Revision < 0 || (input.Action != "pause" && input.Action != "resume" && input.Action != "step") {
		return nil, ErrDebugConflict
	}
	state, err := readDebug(ctx, tx, run)
	if err != nil {
		return nil, err
	}
	if state.Revision != input.Revision {
		return nil, ErrDebugConflict
	}
	if input.Action == "step" && (!state.Paused || state.Active > 0 || state.StepRequested || state.StepNode != "") {
		return nil, ErrDebugConflict
	}
	state.Paused = input.Action != "resume"
	state.StepRequested = input.Action == "step"
	state.StepNode = ""
	state.Revision++
	_, err = tx.Exec(ctx, `INSERT INTO pangu.run_debug(run_id,paused,step_requested,revision) VALUES($1,$2,$3,$4)
 ON CONFLICT(run_id) DO UPDATE SET paused=$2,step_requested=$3,step_node='',revision=$4,updated_at=now()`, run, state.Paused, state.StepRequested, state.Revision)
	return state, err
}
