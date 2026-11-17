package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/jackc/pgx/v5"
)

func (h handler) experienceRunLineage(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if run == nil || !businessPurpose(run.Purpose) {
		problem(w, 404, "运行不存在")
		return
	}
	if !h.allowed(w, r, run.ProjectID, "read", run.CreatedBy) {
		return
	}
	actor := identity.Current(r.Context())
	if actor.ID == "" || actor.Namespace != h.backend.Namespace() {
		problem(w, 403, "需要当前项目身份")
		return
	}
	query, ok := lineageQuery(w, r)
	if !ok {
		return
	}
	scope := experience.Scope{Namespace: h.backend.Namespace(), ProjectID: run.ProjectID, ActorID: actor.ID}
	store := learning.NewStore()
	store.VerifyRun = verifyExperienceRun
	result, err := store.ReadRunLineage(r.Context(), scope, query)
	if !experienceError(w, err) {
		write(w, 200, result)
	}
}

func lineageQuery(w http.ResponseWriter, r *http.Request) (experience.RunLineageQuery, bool) {
	q := experience.RunLineageQuery{ActualRunID: r.PathValue("id"), UsagePage: experience.Page{Limit: 50}, ArtifactPage: experience.Page{Limit: 50}}
	for key, values := range r.URL.Query() {
		if len(values) != 1 || (key != "usage_cursor" && key != "artifact_cursor" && key != "limit") {
			problem(w, 400, "运行经验只接受独立游标和 limit")
			return q, false
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			problem(w, 400, "limit 必须在 1 到 100 之间")
			return q, false
		}
		q.UsagePage.Limit, q.ArtifactPage.Limit = n, n
	}
	q.UsagePage.AfterID = r.URL.Query().Get("usage_cursor")
	q.ArtifactPage.AfterID = r.URL.Query().Get("artifact_cursor")
	return q, true
}

// Acceptance freezes generation 1 under the request ID; a successor has a new request ID.
// Never infer an actual-run binding from caller-provided request IDs or newest watermarks.
func verifyExperienceRun(ctx context.Context, tx pgx.Tx, scope experience.Scope, q experience.RunLineageQuery) (experience.RunBinding, error) {
	var result experience.RunBinding
	var purpose, workflowProject string
	var workflowPresent bool
	err := tx.QueryRow(ctx, `SELECT namespace,project_id::text,run_id::text,id::text,coalesce(snapshot->>'purpose',''),coalesce(snapshot->'workflow'->>'project_id',''),
 coalesce(jsonb_typeof(snapshot#>'{workflow,nodes}')='array' AND snapshot#>'{workflow,nodes}'<>'[]'::jsonb,false)
 FROM pangu.submissions WHERE namespace=$1 AND project_id=$2 AND run_id=$3 FOR SHARE`, scope.Namespace, scope.ProjectID, q.ActualRunID).Scan(&result.Namespace, &result.ProjectID, &result.ActualRunID, &result.RequestID, &purpose, &workflowProject, &workflowPresent)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, experience.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	if purpose == "" && !workflowPresent {
		return result, experience.ErrNotFound
	}
	if !businessPurpose(purpose) || workflowProject != result.ProjectID {
		return result, experience.ErrForbidden
	}
	result.WatermarkGeneration = 1
	return result, nil
}

func (h handler) experienceArtifact(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok {
		return
	}
	if len(r.URL.Query()) != 0 {
		problem(w, 400, "依赖摘要不接受额外参数")
		return
	}
	result, err := learning.NewStore().ReadArtifactSummary(r.Context(), scope, r.PathValue("artifact"))
	if !experienceError(w, err) {
		write(w, 200, result)
	}
}
