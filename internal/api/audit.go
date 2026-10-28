package api

import (
	"github.com/Patrick08T/Pangu/internal/audit"
	"net/http"
)

func (h handler) auditList(w http.ResponseWriter, r *http.Request) {
	for key := range r.URL.Query() {
		if key != "offset" && key != "project_id" {
			problem(w, 400, "只接受 offset 和 project_id")
			return
		}
	}
	offset, err := catalogQuery(r.URL.Query(), false)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	project := r.URL.Query().Get("project_id")
	action := "manage_members"
	if project == "" {
		action = "admin"
	}
	if !h.allowed(w, r, project, action, "") {
		return
	}
	items, err := audit.List(r.Context(), h.backend.Namespace(), project, offset)
	if backendError(w, err) {
		return
	}
	out := page[audit.Entry]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		out.NextOffset = &next
	}
	write(w, 200, out)
}
