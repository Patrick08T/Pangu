package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/google/uuid"
)

func (h handler) experienceRevoke(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "configure")
	if !ok || !experienceVersionID(w, r) {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 1024 {
		problem(w, 400, "撤销原因不能为空且不得超过 1024 字节")
		return
	}
	if !experienceError(w, learning.NewStore().Revoke(r.Context(), scope, r.PathValue("version"), input.Reason)) {
		w.WriteHeader(204)
	}
}

func (h handler) experienceActivate(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "configure")
	if !ok || !experienceVersionID(w, r) {
		return
	}
	var input struct {
		CheckerID  string `json:"checker_id"`
		IntentID   string `json:"intent_id"`
		Generation int64  `json:"generation"`
	}
	if !decode(w, r, &input) {
		return
	}
	if _, err := uuid.Parse(input.CheckerID); err != nil {
		problem(w, 400, "无效的检查器 ID")
		return
	}
	if _, err := uuid.Parse(input.IntentID); err != nil || input.Generation < 1 {
		problem(w, 400, "无效的学习意图或代际")
		return
	}
	request, err := learning.CheckClaim(r.Context(), scope, r.PathValue("version"), input.CheckerID, input.IntentID, input.Generation)
	if experienceError(w, err) {
		return
	}
	result, err := learning.NewStore().Activate(r.Context(), scope, r.PathValue("version"), request)
	if !experienceError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) experienceDelivery(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "configure")
	if !ok || !experienceVersionID(w, r) {
		return
	}
	var input struct {
		ExpectedRevision *int64                    `json:"expected_revision"`
		Policy           experience.DeliveryPolicy `json:"policy"`
		EvidenceID       string                    `json:"evidence_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		problem(w, 400, "投放修改必须提供当前修订")
		return
	}
	store, ready := h.deliveryStore()
	if !ready {
		problem(w, 503, "投放晋级尚未接通独立收益与观察验证")
		return
	}
	result, err := store.SetDelivery(r.Context(), scope, r.PathValue("version"), experience.DeliveryChange{ExpectedRevision: *input.ExpectedRevision, Policy: input.Policy, EvidenceID: input.EvidenceID})
	if !experienceError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) experienceSource(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "configure")
	if !ok {
		return
	}
	if r.Method == "DELETE" {
		if _, err := uuid.Parse(r.PathValue("source")); err != nil {
			problem(w, 400, "无效来源 ID")
			return
		}
		if !experienceError(w, learning.WithdrawSource(r.Context(), scope, r.PathValue("source"))) {
			w.WriteHeader(204)
		}
		return
	}
	var input struct {
		ID        string    `json:"id"`
		Kind      string    `json:"kind"`
		RunID     string    `json:"run_id"`
		NodeID    string    `json:"node_id"`
		Hash      string    `json:"hash"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if !decode(w, r, &input) {
		return
	}
	source := experience.Source{ID: input.ID, Kind: input.Kind, RunID: input.RunID, NodeID: input.NodeID, Hash: input.Hash, ProjectID: scope.ProjectID}
	if !experienceError(w, learning.GrantSource(r.Context(), scope, source, input.ExpiresAt)) {
		w.WriteHeader(204)
	}
}
