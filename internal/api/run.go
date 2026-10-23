package api

import (
	"net/http"
	"time"

	"github.com/Patrick08T/Pangu/internal/scheduler"
)

func (h handler) listRuns(w http.ResponseWriter, r *http.Request) {
	query, err := scheduler.ParseRunQuery(r.URL.Query(), time.Now().UTC())
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	if query.ProjectID != "" && !h.allowed(w, r, query.ProjectID, "read", "") {
		return
	}
	result, err := h.backend.ListRuns(r.Context(), query)
	if err == nil {
		err = h.runList(r.Context(), result)
	}
	if !backendError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) cancelRun(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.runAllowed(w, r, "manage_run") {
		return
	}
	result, err := h.backend.CancelRun(r.Context(), r.PathValue("id"))
	if !backendError(w, err) {
		write(w, 202, result)
	}
}
