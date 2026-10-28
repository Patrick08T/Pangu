package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/identity"
	"io"
	"mime"
	"net/http"

	contract "github.com/Patrick08T/Pangu/api"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

type Backend interface {
	Namespace() string
	Retention(context.Context, bool) ([]workspace.RetentionItem, error)
	RegisterProject(context.Context, workspace.Project) (*workspace.Project, error)
	Projects(context.Context) ([]workspace.Project, error)
	Check(context.Context) error
	Submit(context.Context, workflow.Definition, string) (*submission.Record, error)
	GetRequest(context.Context, string) (*submission.Record, error)
	Rerun(context.Context, string, string) (*submission.Record, error)
	Fork(context.Context, string, string, workflow.ForkInput) (*submission.Record, error)
	GetRun(context.Context, string) (*scheduler.RunDetails, error)
	Events(context.Context, string, string, int, int) (*scheduler.EventPage, error)
	ListRuns(context.Context, scheduler.RunQuery) (*scheduler.RunList, error)
	CancelRun(context.Context, string) (*scheduler.Cancellation, error)
	Preflight(context.Context, workflow.Definition) (*workflow.Preflight, error)
	CodexModels(context.Context) ([]workflow.ModelChoice, error)
	CreateCron(context.Context, workflow.Cron) (*schedule.Plan, error)
	UpdateCron(context.Context, string, int, workflow.Cron, bool) (*schedule.Plan, error)
	ListCrons(context.Context, int64) ([]schedule.Plan, error)
	GetCron(context.Context, string) (*schedule.Plan, error)
	DeleteCron(context.Context, string) error
	CronFires(context.Context, string, int64) ([]schedule.Fire, error)
}

type handler struct {
	access        Access
	backend       Backend
	checkDatabase func(context.Context) error
}

func New(backend Backend, access Access) http.Handler {
	if access == nil {
		panic("API 必须提供身份认证和授权服务")
	}
	h := handler{backend: backend, access: access, checkDatabase: database.Check}
	mux := http.NewServeMux()
	h.catalogRoutes(mux)
	h.factoryRoutes(mux)
	h.identityRoutes(mux)
	h.evaluationRoutes(mux)
	h.deliveryRoutes(mux)
	h.webhookRoutes(mux)
	h.traceRoutes(mux)
	mux.HandleFunc("GET /runs/{id}/usage", h.usage)
	mux.HandleFunc("GET /runs/{id}/approvals", h.approvals)
	mux.HandleFunc("GET /runs/{id}/reports", h.reports)
	mux.HandleFunc("PUT /runs/{id}/reports/{report}/claims/{claim}", h.claimReview)
	mux.HandleFunc("PUT /approvals/{id}", h.decideApproval)
	mux.HandleFunc("GET /audit", h.auditList)
	mux.HandleFunc("GET /retention", h.retention)
	mux.HandleFunc("POST /retention", h.retention)
	mux.HandleFunc("GET /retention/history", h.retentionHistory)
	mux.HandleFunc("POST /projects", h.registerProject)
	mux.HandleFunc("GET /projects", h.projects)
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(contract.OpenAPI)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("POST /workflows/validate", h.validate)
	mux.HandleFunc("POST /workflows/preflight", h.preflight)
	mux.HandleFunc("POST /runs", h.submit)
	mux.HandleFunc("GET /runs", h.listRuns)
	mux.HandleFunc("GET /requests/{id}", h.getRequest)
	mux.HandleFunc("GET /runs/{id}", h.getRun)
	mux.HandleFunc("GET /runs/{id}/attempts", h.attempts)
	mux.HandleFunc("GET /runs/{id}/events", h.events)
	mux.HandleFunc("GET /runs/{id}/nodes/{node}/attempts/{attempt}", h.attempt)
	mux.HandleFunc("GET /runs/{id}/artifacts", h.artifacts)
	mux.HandleFunc("GET /runs/{id}/artifacts.zip", h.exportArtifacts)
	mux.HandleFunc("GET /runs/{id}/artifacts/{artifact}", h.artifact)
	mux.HandleFunc("GET /runs/{id}/artifacts/{artifact}/diff", h.artifact)
	mux.HandleFunc("POST /runs/{id}/cancel", h.cancelRun)
	mux.HandleFunc("POST /runs/{id}/rerun", h.rerun)
	mux.HandleFunc("POST /runs/{id}/fork", h.fork)
	mux.HandleFunc("POST /runs/{id}/fork/preview", h.forkPreview)
	mux.HandleFunc("GET /runs/{id}/inputs", h.runInputs)
	mux.HandleFunc("POST /workflows/preview", h.inputPreview)
	mux.HandleFunc("POST /crons", h.createCron)
	mux.HandleFunc("GET /crons", h.listCrons)
	mux.HandleFunc("GET /crons/preview", h.cronPreview)
	mux.HandleFunc("GET /crons/{id}", h.getCron)
	mux.HandleFunc("PUT /crons/{id}", h.editCron)
	mux.HandleFunc("GET /crons/{id}/fires", h.cronFires)
	mux.HandleFunc("DELETE /crons/{id}", h.deleteCron)
	return observe(mux, access)
}

func (h handler) submit(w http.ResponseWriter, r *http.Request) {
	var d workflow.Definition
	if !decode(w, r, &d) {
		return
	}
	if _, err := d.Layers(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if err := d.ValidateInput(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.PublicKey(key) || d.ProjectID == "" {
		problem(w, 400, "提交需要已登记的 project_id 和唯一的 Idempotency-Key 请求头")
		return
	}
	if !h.allowed(w, r, d.ProjectID, "run", "") {
		return
	}
	result, err := h.backend.Submit(r.Context(), d, key)
	if backendError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/requests/"+result.ID)
	write(w, 202, result)
}

func (h handler) getRequest(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	out, err := h.backend.GetRequest(r.Context(), r.PathValue("id"))
	if !backendError(w, err) && h.allowed(w, r, out.ProjectID, "read", out.CreatedBy) {
		write(w, 200, out)
	}
}

func (h handler) getRun(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	out, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if !backendError(w, err) && h.allowed(w, r, out.ProjectID, "read", out.CreatedBy) {
		write(w, 200, out)
	}
}

func (h handler) createCron(w http.ResponseWriter, r *http.Request) {
	var c workflow.Cron
	if !decode(w, r, &c) {
		return
	}
	if c.Workflow.VersionID != "" {
		resolved, err := h.resolveCron(r.Context(), "", c, false)
		if backendError(w, err) {
			return
		}
		c = resolved
	}
	if err := c.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.allowed(w, r, c.Workflow.ProjectID, "schedule", "") {
		return
	}
	out, err := h.backend.CreateCron(r.Context(), c)
	if !backendError(w, err) {
		write(w, 201, out)
	}
}

func (h handler) listCrons(w http.ResponseWriter, r *http.Request) {
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
	if project != "" && !h.allowed(w, r, project, "read", "") {
		return
	}
	out, err := h.backend.ListCrons(r.Context(), int64(offset))
	if backendError(w, err) {
		return
	}
	result, err := h.cronPage(r.Context(), out, int64(offset), project)
	if !backendError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) deleteCron(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.cronAllowed(w, r, "manage_schedule") {
		return
	}
	if !backendError(w, h.backend.DeleteCron(r.Context(), r.PathValue("id"))) {
		w.WriteHeader(204)
	}
}

func decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		problem(w, 415, "Content-Type 必须为 application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(dest)
	if err == nil {
		var extra any
		if next := decoder.Decode(&extra); next != io.EOF {
			err = next
			if err == nil {
				err = fmt.Errorf("仅允许一个 JSON 对象")
			}
		}
	}
	if err == nil {
		return true
	}
	var sizeError *http.MaxBytesError
	if errors.As(err, &sizeError) {
		problem(w, 413, "请求体超过 256 KiB")
	} else {
		problem(w, 400, "无效 JSON: "+err.Error())
	}
	return false
}

func validID(w http.ResponseWriter, r *http.Request) bool {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		problem(w, 400, "ID 必须为 UUID")
		return false
	}
	r.SetPathValue("id", id.String())
	return true
}

func backendError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, workspace.ErrExpired) {
		problem(w, 410, err.Error())
		return true
	}
	if errors.Is(err, identity.ErrUnauthenticated) {
		problem(w, 401, err.Error())
		return true
	}
	if errors.Is(err, identity.ErrForbidden) {
		problem(w, 403, err.Error())
		return true
	}
	if errors.Is(err, catalog.ErrInvalid) || errors.Is(err, identity.ErrInvalid) {
		problem(w, 400, err.Error())
		return true
	}
	if errors.Is(err, catalog.ErrNotFound) {
		problem(w, 404, "配置对象或版本不存在")
		return true
	}
	if errors.Is(err, catalog.ErrConflict) || errors.Is(err, catalog.ErrArchived) || errors.Is(err, schedule.ErrConflict) {
		problem(w, 409, err.Error())
		return true
	}
	if err == nil {
		return false
	}
	if errors.Is(err, scheduler.ErrNotFound) || errors.Is(err, submission.ErrNotFound) || errors.Is(err, schedule.ErrNotFound) {
		problem(w, 404, err.Error())
		return true
	}
	if errors.Is(err, scheduler.ErrNotReady) {
		problem(w, 503, err.Error())
		return true
	}
	if errors.Is(err, submission.ErrConflict) || errors.Is(err, submission.ErrExpired) || errors.Is(err, scheduler.ErrActive) {
		problem(w, 409, err.Error())
		return true
	}
	problem(w, 502, "后端请求失败，请检查服务连接、资源 ID 和服务端日志")
	return true
}

func problem(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]string{"error": message})
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
