package api

import (
	"github.com/Patrick08T/Pangu/internal/evidence"
	"net/http"
)

func (h handler) runEvidence(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) || !h.allowed(w, r, run.ProjectID, "read", run.CreatedBy) || !logsAvailable(w, r) {
		return
	}
	for key, values := range r.URL.Query() {
		if key != "path" || len(values) != 1 || values[0] == "" {
			problem(w, 400, "仅接受单个证据 path")
			return
		}
	}
	bundle, err := evidence.Load(r.Context(), run.ID, run.ProjectID)
	if backendError(w, err) {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		write(w, 200, bundle.Manifest())
		return
	}
	for _, file := range bundle.Files {
		if file.Path == path {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write([]byte(file.Body))
			return
		}
	}
	problem(w, 404, "当前运行证据索引中没有此文件")
}
