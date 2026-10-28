package api

import (
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"net/http"
)

func (h handler) retention(w http.ResponseWriter, r *http.Request) {
	if !h.allowed(w, r, "", "admin", "") {
		return
	}
	if len(r.URL.Query()) != 0 || r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
		problem(w, 400, "清理接口不接受请求体或查询参数")
		return
	}
	items, err := h.backend.Retention(r.Context(), r.Method == "GET")
	if !backendError(w, err) {
		write(w, 200, map[string]any{"items": items})
	}
}

func (h handler) retentionHistory(w http.ResponseWriter, r *http.Request) {
	if !h.allowed(w, r, "", "admin", "") {
		return
	}
	offset, limit, err := execution.ParsePage(r.URL.Query())
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := workspace.RetentionHistory(r.Context(), h.backend.Namespace(), offset, limit)
	if backendError(w, err) {
		return
	}
	out := page[workspace.RetentionRecord]{Items: items}
	if len(items) > limit {
		next := int64(offset + limit)
		out.NextOffset, out.Items = &next, items[:limit]
	}
	write(w, 200, out)
}
