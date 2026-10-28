package api

import (
	"github.com/Patrick08T/Pangu/internal/workflow"
	"net/http"
)

func (h handler) codexModels(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if len(query) != 0 {
		problem(w, 400, "模型目录不接受查询参数")
		return
	}
	if !h.allowed(w, r, "", "read_catalog", "") {
		return
	}
	items, err := h.backend.CodexModels(r.Context())
	if backendError(w, err) {
		return
	}
	write(w, 200, page[workflow.ModelChoice]{Items: items})
}
