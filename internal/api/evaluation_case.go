package api

import (
	"crypto/sha256"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"net/http"
)

func (h handler) evaluationCases(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		h.createEvaluationCase(w, r)
		return
	}
	for key := range r.URL.Query() {
		if key != "project_id" && key != "offset" {
			problem(w, 400, "只接受 project_id 和 offset")
			return
		}
	}
	offset, err := catalogQuery(r.URL.Query(), false)
	project := r.URL.Query().Get("project_id")
	if err != nil || project == "" {
		problem(w, 400, "必须提供有效 project_id 和非负 offset")
		return
	}
	if !h.allowed(w, r, project, "read", "") {
		return
	}
	items, err := evaluation.Cases(r.Context(), h.backend.Namespace(), project, offset)
	if evaluationError(w, err) {
		return
	}
	result := page[evaluation.Case]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		result.NextOffset = &next
	}
	write(w, 200, result)
}

func (h handler) createEvaluationCase(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID string `json:"project_id"`
		evaluation.CaseInput
	}
	if !decode(w, r, &input) {
		return
	}
	if _, err := uuid.Parse(input.ProjectID); err != nil {
		problem(w, 400, "project_id 必须为 UUID")
		return
	}
	if !h.allowed(w, r, input.ProjectID, "configure", "") {
		return
	}
	item, err := evaluation.CreateCase(r.Context(), h.backend.Namespace(), input.ProjectID, identity.Current(r.Context()).ID, input.CaseInput)
	if !evaluationError(w, err) {
		write(w, 201, item)
	}
}

func (h handler) caseAllowed(w http.ResponseWriter, r *http.Request, action string) *evaluation.Case {
	if !validID(w, r) {
		return nil
	}
	item, err := evaluation.ReadCase(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if evaluationError(w, err) || !h.allowed(w, r, item.ProjectID, action, item.ActorID) {
		return nil
	}
	return item
}

func (h handler) evaluationCase(w http.ResponseWriter, r *http.Request) {
	action := "read"
	if r.Method == "DELETE" {
		action = "manage_run"
	}
	item := h.caseAllowed(w, r, action)
	if item == nil {
		return
	}
	if r.Method == "GET" {
		write(w, 200, item)
		return
	}
	if !evaluationError(w, evaluation.RemoveCase(r.Context(), h.backend.Namespace(), item.ID)) {
		w.WriteHeader(204)
	}
}

func (h handler) evaluationRuns(w http.ResponseWriter, r *http.Request) {
	action := "read"
	if r.Method == "POST" {
		action = "run"
	}
	item := h.caseAllowed(w, r, action)
	if item == nil {
		return
	}
	if r.Method == "POST" {
		h.runEvaluation(w, r, item)
		return
	}
	offset, err := catalogQuery(r.URL.Query(), true)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := evaluation.CaseRuns(r.Context(), h.backend.Namespace(), item.ID, offset)
	if evaluationError(w, err) {
		return
	}
	out := page[evaluation.CaseRun]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		out.NextOffset = &next
	}
	write(w, 200, out)
}

func (h handler) runEvaluation(w http.ResponseWriter, r *http.Request, item *evaluation.Case) {
	var input struct {
		VersionID string `json:"version_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	if _, err := uuid.Parse(input.VersionID); err != nil {
		problem(w, 400, "version_id 必须为 UUID")
		return
	}
	rawKey := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.ValidKey(rawKey) {
		problem(w, 400, "需要唯一有效的 Idempotency-Key")
		return
	}
	if item.PayloadExpired {
		evaluationError(w, evaluation.ErrExpired)
		return
	}
	version, err := catalog.ReadVersion(r.Context(), h.backend.Namespace(), input.VersionID)
	if backendError(w, err) {
		return
	}
	if version.Kind != "agent" && version.Kind != "workflow" {
		problem(w, 400, "请选择 Agent 或工作流版本")
		return
	}
	key := fmt.Sprintf("eval:%x", sha256.Sum256([]byte(item.ID+":"+rawKey)))
	receipt, err := h.versionSubmission(r.Context(), input.VersionID, item.ProjectID, item.Input, key)
	if backendError(w, err) {
		return
	}
	if evaluationError(w, evaluation.LinkRun(r.Context(), h.backend.Namespace(), item.ID, input.VersionID, receipt.ID)) {
		return
	}
	w.Header().Set("Location", "/api/requests/"+receipt.ID)
	write(w, 202, receipt)
}
