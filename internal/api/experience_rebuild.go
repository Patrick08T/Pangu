package api

import (
	"context"
	"net/http"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

type rebuildProvider interface {
	RebuildInvalidated(context.Context, string, string) (*submission.Record, error)
}

type rebuildSummary struct {
	approval.RebuildIntent
	SuccessorRequestID string `json:"successor_request_id,omitempty"`
	CanRebuild         bool   `json:"can_rebuild"`
	BlockedReason      string `json:"blocked_reason,omitempty"`
}

func (h handler) experienceRebuilds(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.runAllowed(w, r, "read") {
		return
	}
	intents, err := approval.RebuildIntents(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if approvalError(w, err) {
		return
	}
	canRebuild, blockedReason, err := h.rebuildAvailability(r)
	if backendError(w, err) {
		return
	}
	result := []rebuildSummary{}
	for _, intent := range intents {
		successor, err := approval.RebuildSuccessor(r.Context(), h.backend.Namespace(), intent.ID)
		if approvalError(w, err) {
			return
		}
		result = append(result, rebuildSummary{RebuildIntent: intent, SuccessorRequestID: successor, CanRebuild: canRebuild, BlockedReason: blockedReason})
	}
	write(w, 200, result)
}

func (h handler) experienceRebuild(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.runAllowed(w, r, "run") {
		return
	}
	if identity.Current(r.Context()).TokenID != "" {
		problem(w, 403, "重建需要原创建人的交互会话")
		return
	}
	if _, err := uuid.Parse(r.PathValue("intent")); err != nil {
		problem(w, 400, "重建意图 ID 无效")
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	backend, ok := h.backend.(rebuildProvider)
	if !ok {
		problem(w, 503, "失效运行重建尚未接通")
		return
	}
	result, err := backend.RebuildInvalidated(r.Context(), r.PathValue("id"), r.PathValue("intent"))
	if approvalError(w, err) {
		return
	}
	if result == nil || result.ID == "" {
		problem(w, 503, "重建未产生有效后继请求")
		return
	}
	w.Header().Set("Location", "/api/requests/"+result.ID)
	write(w, 202, result)
}
