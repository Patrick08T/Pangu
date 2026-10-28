package api

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"net/http"
)

type trialInput struct {
	ProjectID string `json:"project_id"`
	Revision  int    `json:"revision"`
	Input     string `json:"input"`
}

func (h handler) agentDraft(w http.ResponseWriter, r *http.Request) *catalog.Object {
	if !validID(w, r) || !h.catalogAllowed(w, r, false, "read") {
		return nil
	}
	item, err := catalog.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if backendError(w, err) {
		return nil
	}
	if item.Kind != "agent" {
		problem(w, 400, "仅支持 Agent 草稿")
		return nil
	}
	return item
}

func (h handler) draftPreflight(w http.ResponseWriter, r *http.Request) {
	item := h.agentDraft(w, r)
	if item == nil {
		return
	}
	var input trialInput
	if !decode(w, r, &input) {
		return
	}
	if !h.allowed(w, r, input.ProjectID, "run", "") {
		return
	}
	d, err := catalog.CompileDraft(r.Context(), h.backend.Namespace(), item.ID, input.Revision, input.ProjectID, input.Input)
	if !backendError(w, err) {
		h.inspectWorkflow(w, r, *d)
	}
}

func (h handler) agentTrials(w http.ResponseWriter, r *http.Request) {
	item := h.agentDraft(w, r)
	if item == nil {
		return
	}
	if r.Method == "POST" {
		h.runTrial(w, r, item)
		return
	}
	project := r.URL.Query().Get("project_id")
	if !h.allowed(w, r, project, "read", "") {
		return
	}
	query := r.URL.Query()
	query.Del("project_id")
	offset, err := catalogQuery(query, true)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := submission.Trials(r.Context(), h.backend.Namespace(), item.ID, project, offset)
	if backendError(w, err) {
		return
	}
	result := page[submission.Trial]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		result.NextOffset = &next
	}
	write(w, 200, result)
}

func (h handler) runTrial(w http.ResponseWriter, r *http.Request, item *catalog.Object) {
	var input trialInput
	if !decode(w, r, &input) {
		return
	}
	if !h.allowed(w, r, input.ProjectID, "run", "") {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if input.Revision < 1 || len(r.Header.Values("Idempotency-Key")) != 1 || !submission.PublicKey(key) {
		problem(w, 400, "需要有效修订号和唯一的 Idempotency-Key")
		return
	}
	if err := submission.CheckTrialKey(key); err != nil {
		if errors.Is(err, submission.ErrExpired) {
			backendError(w, err)
		} else {
			problem(w, 400, err.Error())
		}
		return
	}
	accepted, err := h.trialSubmission(r.Context(), item, input, key)
	if !backendError(w, err) {
		w.Header().Set("Location", "/api/requests/"+accepted.ID)
		write(w, 202, accepted)
	}
}

func (h handler) trialSubmission(ctx context.Context, item *catalog.Object, input trialInput, key string) (*submission.Record, error) {
	namespace := h.backend.Namespace()
	saved, err := submission.FindTrial(ctx, namespace, input.ProjectID, item.ID, input.Revision, input.Input, key)
	if err == nil {
		return saved, nil
	}
	if !errors.Is(err, submission.ErrNotFound) {
		return nil, err
	}
	d, err := catalog.CompileDraft(ctx, namespace, item.ID, input.Revision, input.ProjectID, input.Input)
	if err != nil {
		return nil, err
	}
	result, err := h.backend.Preflight(ctx, *d)
	if err != nil {
		return nil, err
	}
	if !result.Ready {
		return nil, scheduler.ErrNotReady
	}
	saved, err = submission.PrepareTrial(ctx, namespace, item.ID, input.Revision, *d, key)
	if err != nil {
		return nil, err
	}
	// 提交已落库，派发暂时失败由现有对账恢复，客户端仍能查询回执。
	accepted, err := h.backend.Submit(ctx, *d, submission.TrialKey(key))
	if err == nil {
		return accepted, nil
	}
	return saved, nil
}
