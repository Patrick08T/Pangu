package api

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/delivery"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"net/http"
)

func (h handler) deliveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /deliveries/{id}/artifacts.zip", h.exportDelivery)
	mux.HandleFunc("POST /runs/{id}/reports/{report}/deliveries", h.createDelivery)
	mux.HandleFunc("GET /deliveries", h.deliveries)
	mux.HandleFunc("GET /deliveries/{id}", h.getDelivery)
	mux.HandleFunc("POST /deliveries/{id}/run", h.runDelivery)
}

func (h handler) createDelivery(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	report, err := uuid.Parse(r.PathValue("report"))
	if err != nil {
		problem(w, 400, "报告 ID 无效")
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if !h.allowed(w, r, run.ProjectID, "configure", "") {
		return
	}
	if !run.StopConfirmed || run.Status != "succeeded" || run.LogsExpired {
		problem(w, 409, "需要已成功结束且未过期的设计报告")
		return
	}
	if !h.deliveryVerification(w, r, run.ProjectID) {
		return
	}
	var input delivery.CreateInput
	if !decode(w, r, &input) {
		return
	}
	for _, id := range []string{input.ImplementationVersion, input.ReviewVersion} {
		if _, err = uuid.Parse(id); err != nil {
			problem(w, 400, "Agent 版本 ID 无效")
			return
		}
	}
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.ValidKey(key) {
		problem(w, 400, "需要唯一有效的 Idempotency-Key")
		return
	}
	item, err := delivery.Create(r.Context(), h.backend.Namespace(), run.ProjectID, identity.Current(r.Context()).ID, run.ID, report.String(), key, input)
	if !deliveryError(w, err) {
		write(w, 201, item)
	}
}

func (h handler) deliveryVerification(w http.ResponseWriter, r *http.Request, project string) bool {
	item, err := workspace.ProjectByID(r.Context(), h.backend.Namespace(), project)
	if backendError(w, err) {
		return false
	}
	if len(item.Verification) == 0 {
		problem(w, 409, "交付项目需要先登记真实验证命令")
		return false
	}
	return true
}

func (h handler) deliveries(w http.ResponseWriter, r *http.Request) {
	for key := range r.URL.Query() {
		if key != "project_id" && key != "offset" {
			problem(w, 400, "只接受 project_id 和 offset")
			return
		}
	}
	project := r.URL.Query().Get("project_id")
	offset, err := catalogQuery(r.URL.Query(), false)
	if _, idErr := uuid.Parse(project); err != nil || idErr != nil {
		problem(w, 400, "需要 project_id 和非负 offset")
		return
	}
	if !h.allowed(w, r, project, "read", "") {
		return
	}
	items, err := delivery.List(r.Context(), h.backend.Namespace(), project, offset)
	if deliveryError(w, err) {
		return
	}
	result := page[delivery.Draft]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		result.NextOffset = &next
	}
	write(w, 200, result)
}

func (h handler) deliveryAllowed(w http.ResponseWriter, r *http.Request, action string) *delivery.Draft {
	if !validID(w, r) {
		return nil
	}
	item, err := delivery.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if deliveryError(w, err) || !h.allowed(w, r, item.ProjectID, action, item.ActorID) {
		return nil
	}
	return item
}

func (h handler) getDelivery(w http.ResponseWriter, r *http.Request) {
	if item := h.deliveryAllowed(w, r, "read"); item != nil {
		write(w, 200, item)
	}
}

func (h handler) runDelivery(w http.ResponseWriter, r *http.Request) {
	item := h.deliveryAllowed(w, r, "manage_run")
	if item == nil {
		return
	}
	var input struct {
		SubjectHash string `json:"subject_hash"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.SubjectHash != item.SubjectHash {
		deliveryError(w, delivery.ErrConflict)
		return
	}
	var receipt *submission.Record
	var err error
	if item.RequestID != "" {
		receipt, err = submission.Get(r.Context(), h.backend.Namespace(), item.RequestID)
	} else {
		if item.Definition == nil {
			deliveryError(w, delivery.ErrExpired)
			return
		}
		if !h.deliveryVerification(w, r, item.ProjectID) {
			return
		}
		if deliveryError(w, delivery.Claim(r.Context(), h.backend.Namespace(), item, identity.Current(r.Context()).ID)) {
			return
		}
		receipt, err = h.backend.Submit(r.Context(), *item.Definition, "delivery:"+item.ID)
	}
	if err == nil {
		err = delivery.Link(r.Context(), h.backend.Namespace(), item.ID, receipt.ID)
	}
	if deliveryError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/requests/"+receipt.ID)
	write(w, 202, receipt)
}

func deliveryError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{{delivery.ErrInvalid, 400}, {delivery.ErrConflict, 409}, {delivery.ErrNotFound, 404}, {delivery.ErrExpired, 410}} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return researchError(w, err)
}
