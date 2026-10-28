package api

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/google/uuid"
	"net/http"
)

func (h handler) reports(w http.ResponseWriter, r *http.Request) {
	if !h.artifactRun(w, r) {
		return
	}
	items, err := research.Details(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if !researchError(w, err) {
		write(w, 200, items)
	}
}

func (h handler) claimReview(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if _, err := uuid.Parse(r.PathValue("report")); err != nil {
		problem(w, 400, "报告 ID 无效")
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if !h.allowed(w, r, run.ProjectID, "manage_run", run.CreatedBy) {
		return
	}
	if actor := identity.Current(r.Context()); actor.ID == "" || actor.TokenID != "" {
		problem(w, 403, "人工核验需要飞书会话")
		return
	}
	var input research.ClaimReviewInput
	if !decode(w, r, &input) {
		return
	}
	item, err := research.Review(r.Context(), h.backend.Namespace(), run.ID, r.PathValue("report"), r.PathValue("claim"), identity.Current(r.Context()), input)
	if !researchError(w, err) {
		write(w, 200, item)
	}
}

func researchError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{{research.ErrNotFound, 404}, {research.ErrReviewInvalid, 400}, {research.ErrReviewConflict, 409}} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return backendError(w, err)
}
