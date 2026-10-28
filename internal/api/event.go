package api

import (
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/google/uuid"
	"net/http"
)

func (h handler) events(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	query := r.URL.Query()
	node := query.Get("node_run_id")
	if values, ok := query["node_run_id"]; ok {
		if _, err := uuid.Parse(node); err != nil || len(values) != 1 {
			problem(w, 400, "node_run_id 必须为唯一 UUID")
			return
		}
		query.Del("node_run_id")
	}
	offset, limit, err := execution.ParsePage(query)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.runAllowed(w, r, "read") {
		return
	}
	page, err := h.backend.Events(r.Context(), r.PathValue("id"), node, offset, limit)
	if !backendError(w, err) {
		write(w, 200, page)
	}
}
