package api

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/identity"
	"net/http"
)

func (h handler) approvals(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if !h.allowed(w, r, run.ProjectID, "read", run.CreatedBy) {
		return
	}
	items, err := approval.List(r.Context(), h.backend.Namespace(), run.ID)
	if !approvalError(w, err) {
		write(w, 200, items)
	}
}

func (h handler) decideApproval(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	gate, err := approval.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if approvalError(w, err) {
		return
	}
	if !h.allowed(w, r, gate.ProjectID, "manage_run", gate.OwnerID) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), gate.RunID)
	if backendError(w, err) {
		return
	}
	if run.StopConfirmed || run.Status == "cancelling" {
		problem(w, 409, "运行已停止或正在取消，不能继续审批")
		return
	}
	var input approval.Decision
	if !decode(w, r, &input) {
		return
	}
	item, err := approval.Decide(r.Context(), identity.Current(r.Context()), gate.ID, input)
	if !approvalError(w, err) {
		write(w, 200, item)
	}
}

func approvalError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{{approval.ErrNotFound, 404}, {approval.ErrConflict, 409}, {approval.ErrExpired, 410}, {approval.ErrInvalid, 400}} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return backendError(w, err)
}
