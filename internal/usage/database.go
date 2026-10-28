package usage

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"time"
)

type scopeKey struct{}
type scope struct {
	run, node, nodeRun string
	attempt            int
}

func WithRun(ctx context.Context, run, node, nodeRun string, attempt int) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope{run, node, nodeRun, attempt})
}

func Reserve(ctx context.Context, kind, model string, price *workflow.Price) (string, error) {
	s, ok := ctx.Value(scopeKey{}).(scope)
	if !ok {
		return "", nil
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "usage:"+s.run); err != nil {
		return "", err
	}
	var raw []byte
	if err = tx.QueryRow(ctx, "SELECT snapshot->'workflow' FROM pangu.submissions WHERE run_id=$1 AND purged_at IS NULL", s.run).Scan(&raw); err != nil {
		return "", err
	}
	var definition workflow.Definition
	if err = json.Unmarshal(raw, &definition); err != nil {
		return "", err
	}
	budget := definition.Budget
	if err = budget.Validate(); err != nil {
		return "", err
	}
	limit, valid := 0, kind == "model" || kind == "search" || kind == "codex"
	if budget != nil {
		limit = map[string]int{"model": budget.ModelCalls, "search": budget.SearchCalls, "codex": budget.CodexSessions}[kind]
	}
	if !valid || len(model) > 128 {
		return "", errors.New("用量类型或模型无效")
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM pangu.usage_calls WHERE run_id=$1 AND kind=$2", s.run, kind).Scan(&count); err != nil {
		return "", err
	}
	if budget != nil && count >= limit {
		return "", &workflow.Failure{Kind: "budget", Cause: errors.New("调用预算已耗尽；已预留或结果未知的请求不会退回额度")}
	}
	if err = price.Validate(); err != nil {
		return "", err
	}
	id := uuid.NewString()
	var rates []byte
	if price != nil {
		rates, err = json.Marshal(price)
		if err != nil {
			return "", err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.usage_calls(id,run_id,node_id,node_run_id,attempt,kind,model,price) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, s.run, s.node, s.nodeRun, s.attempt, kind, model, rates)
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

type Tokens struct {
	Input  int64
	Output int64
	Cached int64
}

func Estimate(price *workflow.Price, tokens *Tokens) *int64 {
	if price == nil || tokens == nil || price.Validate() != nil || !tokens.valid() {
		return nil
	}
	value := (tokens.Input*price.InputPerMillionMicros + tokens.Output*price.OutputPerMillionMicros + 999999) / 1000000
	return &value
}

func (t *Tokens) valid() bool {
	return t != nil && t.Input >= 0 && t.Input <= 100000000 && t.Output >= 0 && t.Output <= 100000000 && t.Cached >= 0 && t.Cached <= t.Input
}

func Finish(id string, tokens *Tokens, completed bool) error {
	if id == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var raw []byte
	if err = conn.QueryRow(ctx, "SELECT price FROM pangu.usage_calls WHERE id=$1", id).Scan(&raw); err != nil {
		return err
	}
	var price *workflow.Price
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &price); err != nil {
			return err
		}
	}
	var input, output, cached *int64
	status := "unknown"
	if completed {
		status = "completed"
	}
	if tokens.valid() {
		input, output, cached, status = &tokens.Input, &tokens.Output, &tokens.Cached, "reported"
	} else {
		tokens = nil
	}
	_, err = conn.Exec(ctx, `UPDATE pangu.usage_calls SET input_tokens=$2,output_tokens=$3,cached_tokens=$4,estimate_micros=$5,status=$6,finished_at=clock_timestamp() WHERE id=$1 AND finished_at IS NULL`, id, input, output, cached, Estimate(price, tokens), status)
	return err
}
