package api

import (
	"io"
	"net/http"

	"github.com/Patrick08T/Pangu/internal/submission"
)

func (h handler) rerun(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.ValidKey(key) {
		problem(w, 400, "需要有效且唯一的 Idempotency-Key 请求头")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1))
	if err != nil || len(body) != 0 || len(r.URL.Query()) > 0 {
		problem(w, 400, "重新运行不接受覆盖原快照的请求内容")
		return
	}
	if !h.runAllowed(w, r, "run") {
		return
	}
	out, err := h.backend.Rerun(r.Context(), r.PathValue("id"), key)
	if backendError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/requests/"+out.ID)
	write(w, 202, out)
}
