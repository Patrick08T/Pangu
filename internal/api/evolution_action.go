package api

import (
	"net/http"
	"strings"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
)

func (h handler) evolutionAction(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.evolutionScope(w, r)
	if !ok {
		return
	}
	var input struct {
		Generation int    `json:"generation"`
		RunID      string `json:"run_id,omitempty"`
	}
	if !decode(w, r, &input) {
		return
	}
	if _, err := uuid.Parse(r.PathValue("intent")); err != nil || input.Generation < 1 {
		problem(w, 400, "进化意图或代际无效")
		return
	}
	action := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if action == "observe" {
		if _, err := uuid.Parse(input.RunID); err != nil {
			problem(w, 400, "观察运行 ID 无效")
			return
		}
	} else if input.RunID != "" {
		problem(w, 400, "当前操作不接收运行 ID")
		return
	}
	service, ready := h.evolutionRuntime()
	if !ready {
		problem(w, 503, "独立实验执行器尚未接通")
		return
	}
	if action == "decision" {
		decision, err := service.Decide(r.Context(), scope, r.PathValue("intent"), input.Generation)
		if evolutionError(w, err) {
			return
		}
		if decision.Outcome == "" || decision.Category == "" {
			problem(w, 503, "独立判定尚未产生有效结果")
			return
		}
		write(w, 200, decision)
		return
	}
	var result *evolution.Intent
	var err error
	if action == "promote" {
		result, err = service.Promote(r.Context(), scope, r.PathValue("intent"), input.Generation)
	}
	if action == "observe" {
		result, err = service.Observe(r.Context(), scope, r.PathValue("intent"), input.Generation, input.RunID)
	}
	if evolutionError(w, err) {
		return
	}
	if result == nil || result.ID != r.PathValue("intent") || result.Generation != input.Generation || result.State == "" {
		problem(w, 503, "进化操作尚未产生有效结果")
		return
	}
	summary, err := service.ReadIntentSummary(r.Context(), scope, result.ID)
	if !evolutionError(w, err) {
		write(w, 202, summary)
	}
}
