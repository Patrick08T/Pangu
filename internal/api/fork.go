package api

import (
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"net/http"
)

func (h handler) forkPreview(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if !h.allowed(w, r, run.ProjectID, "run", "") {
		return
	}
	if run.LogsExpired || run.Definition == nil {
		problem(w, 410, "来源快照已过期")
		return
	}
	var input workflow.ForkInput
	if !decode(w, r, &input) {
		return
	}
	if input.Workflow.ProjectID != run.ProjectID {
		problem(w, 403, "返工不能更换项目")
		return
	}
	if backendError(w, catalog.CheckDefinition(r.Context(), h.backend.Namespace(), input.Workflow)) {
		return
	}
	reused, err := scheduler.PreviewFork(run, input)
	if backendError(w, err) {
		return
	}
	invalidated := []string{}
	for _, node := range input.Workflow.Nodes {
		found := false
		for _, id := range reused {
			found = found || node.ID == id
		}
		if !found {
			invalidated = append(invalidated, node.ID)
		}
	}
	write(w, 200, map[string]any{"reused_nodes": reused, "execute_nodes": invalidated, "input_changed": input.Workflow.Input != run.Definition.Input})
}

func (h handler) fork(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	var input workflow.ForkInput
	if !decode(w, r, &input) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.ValidKey(key) {
		problem(w, 400, "需要唯一有效的 Idempotency-Key")
		return
	}
	if !h.allowed(w, r, input.Workflow.ProjectID, "run", "") {
		return
	}
	item, err := h.backend.Fork(r.Context(), r.PathValue("id"), key, input)
	if !backendError(w, err) {
		w.Header().Set("Location", "/api/requests/"+item.ID)
		write(w, 202, item)
	}
}

func (h handler) runInputs(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if !h.allowed(w, r, run.ProjectID, "read", run.CreatedBy) {
		return
	}
	if run.LogsExpired || run.Definition == nil {
		problem(w, 410, "运行输入已过保留期或快照不可用")
		return
	}
	out, err := workflow.PreviewInputs(*run.Definition, scheduler.Outputs(run))
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	write(w, 200, out)
}

func (h handler) inputPreview(w http.ResponseWriter, r *http.Request) {
	var d workflow.Definition
	if !decode(w, r, &d) {
		return
	}
	if !h.allowed(w, r, d.ProjectID, "run", "") {
		return
	}
	if diagnostic.Sensitive([]byte(d.Input)) {
		problem(w, 400, "输入疑似含敏感数据")
		return
	}
	out, err := workflow.PreviewInputs(d, nil)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	write(w, 200, out)
}
