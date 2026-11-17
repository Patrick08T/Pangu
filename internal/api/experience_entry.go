package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/Patrick08T/Pangu/internal/submission"
)

type entrySubmitter interface {
	SubmitEntry(context.Context, string, string, string, string) (*submission.Record, error)
}

func (h handler) experienceEntry(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "run")
	if !ok {
		return
	}
	entry := r.PathValue("entry")
	if !submission.ValidKey(entry) || strings.ContainsAny(entry, ":/\\") || len(r.URL.Query()) > 0 {
		problem(w, 400, "仅接受工作台入口的裸 ID，不接受版本或其他入口种类")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.PublicKey(key) {
		problem(w, 400, "需要有效且唯一的 Idempotency-Key 请求头")
		return
	}
	var input struct {
		Input string `json:"input"`
	}
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Input) == "" {
		problem(w, 400, "任务输入不能为空")
		return
	}
	backend, ready := h.backend.(entrySubmitter)
	if !ready {
		problem(w, 503, "工作台真实入口提交尚未接通")
		return
	}
	result, err := backend.SubmitEntry(r.Context(), scope.ProjectID, entry, input.Input, key)
	if result != nil && result.ID != "" {
		w.Header().Set("Location", "/api/requests/"+result.ID)
	}
	if evolutionError(w, err) {
		return
	}
	if result == nil || result.ID == "" || result.ProjectID != scope.ProjectID || result.Status == "" {
		problem(w, 503, "入口尚未建立有效业务请求")
		return
	}
	// A durable pending request is not a completed or successfully executed run.
	write(w, 202, result)
}
