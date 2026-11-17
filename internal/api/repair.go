package api

import (
	"github.com/Patrick08T/Pangu/internal/approval"
	"net/http"
)

func (h handler) prepareRepair(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	gate, err := approval.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if approvalError(w, err) || !h.allowed(w, r, gate.ProjectID, "manage_run", gate.OwnerID) || !h.allowed(w, r, "", "manage_catalog", "") {
		return
	}
	var input approval.RepairInput
	if !decode(w, r, &input) {
		return
	}
	result, err := h.backend.PrepareRepair(r.Context(), gate.ID, input)
	if !approvalError(w, err) {
		write(w, 200, result)
	}
}
