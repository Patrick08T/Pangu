package api

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/webhook"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"io"
	"mime"
	"net/http"
	"os"
)

func (h handler) webhookRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /github/hooks", h.githubHooks)
	mux.HandleFunc("POST /github/hooks", h.createGithubHook)
	mux.HandleFunc("DELETE /github/hooks/{id}", h.disableGithubHook)
	mux.HandleFunc("GET /github/hooks/{id}/events", h.githubEvents)
	mux.HandleFunc("POST /hooks/github/{id}", h.githubEvent)
}

func (h handler) createGithubHook(w http.ResponseWriter, r *http.Request) {
	var input webhook.Config
	if !decode(w, r, &input) {
		return
	}
	if webhookError(w, input.Validate()) || !h.allowed(w, r, input.ProjectID, "manage_connections", "") {
		return
	}
	if len(os.Getenv(input.SecretRef)) < 32 {
		problem(w, 409, "请先在服务端配置至少 32 字节的 Webhook 签名密钥引用")
		return
	}
	item, err := webhook.Create(r.Context(), h.backend.Namespace(), input)
	if !webhookError(w, err) {
		write(w, 201, item)
	}
}

func (h handler) githubHooks(w http.ResponseWriter, r *http.Request) {
	for key := range r.URL.Query() {
		if key != "project_id" && key != "offset" {
			problem(w, 400, "只接受 project_id 和 offset")
			return
		}
	}
	project := r.URL.Query().Get("project_id")
	offset, err := catalogQuery(r.URL.Query(), false)
	if _, idErr := uuid.Parse(project); idErr != nil || err != nil {
		problem(w, 400, "需要 project_id 和有效 offset")
		return
	}
	if !h.allowed(w, r, project, "manage_connections", "") {
		return
	}
	items, err := webhook.List(r.Context(), h.backend.Namespace(), project, offset)
	if webhookError(w, err) {
		return
	}
	result := page[webhook.Hook]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		result.NextOffset = &next
	}
	write(w, 200, result)
}

func (h handler) hookAllowed(w http.ResponseWriter, r *http.Request) *webhook.Hook {
	if !validID(w, r) {
		return nil
	}
	item, err := webhook.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if webhookError(w, err) || !h.allowed(w, r, item.ProjectID, "manage_connections", "") {
		return nil
	}
	return item
}

func (h handler) disableGithubHook(w http.ResponseWriter, r *http.Request) {
	item := h.hookAllowed(w, r)
	if item == nil {
		return
	}
	if !webhookError(w, webhook.Disable(r.Context(), h.backend.Namespace(), item.ID)) {
		w.WriteHeader(204)
	}
}

func (h handler) githubEvents(w http.ResponseWriter, r *http.Request) {
	item := h.hookAllowed(w, r)
	if item == nil {
		return
	}
	offset, err := catalogQuery(r.URL.Query(), true)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := webhook.Events(r.Context(), h.backend.Namespace(), item.ID, offset)
	if webhookError(w, err) {
		return
	}
	result := page[webhook.Event]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		result.NextOffset = &next
	}
	write(w, 200, result)
}

func (h handler) githubEvent(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	hook, err := webhook.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if webhookError(w, err) {
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		problem(w, 415, "需要 application/json")
		return
	}
	for _, name := range []string{"X-Hub-Signature-256", "X-GitHub-Delivery", "X-GitHub-Event"} {
		if len(r.Header.Values(name)) != 1 {
			problem(w, 400, "缺少唯一的 GitHub 事件头")
			return
		}
	}
	eventID, err := uuid.Parse(r.Header.Get("X-GitHub-Delivery"))
	if err != nil {
		problem(w, 400, "事件 ID 无效")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256*1024))
	if err != nil {
		problem(w, 413, "事件体超过 256 KiB 或读取失败")
		return
	}
	if !webhook.Signature(raw, r.Header.Get("X-Hub-Signature-256"), os.Getenv(hook.SecretRef)) {
		problem(w, 401, "事件签名无效")
		return
	}
	if r.Header.Get("X-GitHub-Event") != "issues" {
		problem(w, 400, "仅支持 issues labeled 事件")
		return
	}
	input, err := webhook.Input(raw, hook.Config)
	if webhookError(w, err) {
		return
	}
	h.acceptGithubEvent(w, r, hook, eventID.String(), fmt.Sprintf("%x", sha256.Sum256(raw)), input)
}

func (h handler) acceptGithubEvent(w http.ResponseWriter, r *http.Request, hook *webhook.Hook, event, digest, input string) {
	if !hook.Enabled {
		webhookError(w, webhook.ErrDisabled)
		return
	}
	if backendError(w, identity.Authorize(r.Context(), identity.Actor{ID: hook.ActorID, Namespace: h.backend.Namespace()}, hook.ProjectID, "run", "")) {
		return
	}
	previous, err := webhook.Receipt(r.Context(), h.backend.Namespace(), hook.ID, event, digest)
	if err == nil {
		write(w, 202, previous)
		return
	}
	if !errors.Is(err, webhook.ErrNotFound) {
		webhookError(w, err)
		return
	}
	d, err := catalog.Compile(r.Context(), h.backend.Namespace(), hook.VersionID, hook.ProjectID, input)
	if webhookError(w, err) {
		return
	}
	if webhookError(w, webhookDefinition(*d)) {
		return
	}
	if !hook.Enabled {
		webhookError(w, webhook.ErrDisabled)
		return
	}
	if backendError(w, identity.Authorize(r.Context(), identity.Actor{ID: hook.ActorID, Namespace: h.backend.Namespace()}, hook.ProjectID, "run", "")) {
		return
	}
	check, err := h.backend.Preflight(r.Context(), *d)
	if backendError(w, err) {
		return
	}
	if !check.Ready {
		problem(w, 503, "执行环境尚未就绪")
		return
	}
	item, err := webhook.Accept(r.Context(), h.backend.Namespace(), hook.ID, event, digest, *d)
	if webhookError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/requests/"+item.ID)
	write(w, 202, item)
}

func webhookDefinition(d workflow.Definition) error {
	for _, node := range d.Nodes {
		if node.Executor == "codex" && !node.ReadOnly && node.Approval == nil {
			return webhook.ErrInvalid
		}
	}
	return nil
}

func webhookError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{{webhook.ErrInvalid, 400}, {webhook.ErrNotFound, 404}, {webhook.ErrConflict, 409}, {webhook.ErrDisabled, 403}} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return backendError(w, err)
}
