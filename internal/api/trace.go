package api

import (
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/trace"
	"net/http"
)

func (h handler) traceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /runs/{id}/activity", h.activity)
	mux.HandleFunc("GET /runs/{id}/trace", h.trace)
	mux.HandleFunc("GET /runs/{id}/trace.ndjson", h.exportTrace)
	mux.HandleFunc("GET /runs/{id}/debug", h.debug)
	mux.HandleFunc("POST /runs/{id}/debug", h.controlDebug)
}

func (h handler) activity(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.runAllowed(w, r, "read") || !logsAvailable(w, r) {
		return
	}
	if len(r.URL.Query()) != 0 {
		problem(w, 400, "当前活动查询不接受筛选参数")
		return
	}
	items, err := trace.Activity(r.Context(), r.PathValue("id"))
	if !backendError(w, err) {
		write(w, 200, items)
	}
}

func (h handler) traceQuery(w http.ResponseWriter, r *http.Request) (trace.Query, bool) {
	if !validID(w, r) || !h.runAllowed(w, r, "read") || !logsAvailable(w, r) {
		return trace.Query{}, false
	}
	query, err := trace.ParseQuery(r.URL.Query())
	if err != nil {
		problem(w, 400, err.Error())
		return query, false
	}
	return query, true
}

func (h handler) trace(w http.ResponseWriter, r *http.Request) {
	query, ok := h.traceQuery(w, r)
	if !ok {
		return
	}
	page, err := trace.List(r.Context(), r.PathValue("id"), query)
	if !backendError(w, err) {
		write(w, 200, page)
	}
}

func (h handler) exportTrace(w http.ResponseWriter, r *http.Request) {
	query, ok := h.traceQuery(w, r)
	if !ok {
		return
	}
	query.Limit = 100
	page, err := trace.List(r.Context(), r.PathValue("id"), query)
	if backendError(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="pangu-trace.ndjson"`)
	encoder := json.NewEncoder(w)
	for {
		for _, event := range page.Items {
			if err = encoder.Encode(event); err != nil {
				return
			}
		}
		if page.NextBefore == nil {
			return
		}
		query.Before = *page.NextBefore
		page, err = trace.List(r.Context(), r.PathValue("id"), query)
		if err != nil {
			_ = encoder.Encode(map[string]string{"export_error": "导出中断，文件不完整，请重试"})
			return
		}
	}
}

func (h handler) debug(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.runAllowed(w, r, "read") || !logsAvailable(w, r) {
		return
	}
	state, err := execution.GetDebug(r.Context(), r.PathValue("id"))
	if !backendError(w, err) {
		write(w, 200, state)
	}
}

func (h handler) controlDebug(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) || !h.allowed(w, r, run.ProjectID, "manage_run", run.CreatedBy) {
		return
	}
	if run.StopConfirmed || run.CancelRequested || run.Status == "cancelling" || run.LogsExpired {
		problem(w, 409, execution.ErrDebugConflict.Error())
		return
	}
	var input struct {
		Action   string `json:"action"`
		Revision *int   `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Revision == nil || *input.Revision < 0 || (input.Action != "pause" && input.Action != "resume" && input.Action != "step") {
		problem(w, 400, "无效的调试命令或版本")
		return
	}
	state, err := execution.Control(r.Context(), identity.Current(r.Context()), r.PathValue("id"), execution.DebugCommand{Action: input.Action, Revision: *input.Revision})
	if errors.Is(err, execution.ErrDebugConflict) || errors.Is(err, execution.ErrCancelled) {
		problem(w, 409, err.Error())
		return
	}
	if !backendError(w, err) {
		write(w, 200, state)
	}
}
