package api

import (
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/google/uuid"
	"net/http"
)

func (h handler) factoryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /factory/sessions", h.factorySessions)
	mux.HandleFunc("POST /factory/sessions", h.factorySessions)
	mux.HandleFunc("GET /factory/sessions/{id}", h.factorySession)
	mux.HandleFunc("POST /factory/sessions/{id}/messages", h.factoryMessage)
	mux.HandleFunc("POST /factory/sessions/{id}/apply", h.factoryApply)
}

func (h handler) factorySessions(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var in struct {
			ProjectID string `json:"project_id"`
		}
		if !decode(w, r, &in) || !h.allowed(w, r, in.ProjectID, "configure", "") {
			return
		}
		s, err := factory.Create(r.Context(), h.backend.Namespace(), in.ProjectID)
		if !backendError(w, err) {
			write(w, 201, s)
		}
		return
	}
	project := r.URL.Query().Get("project_id")
	if _, err := uuid.Parse(project); err != nil {
		problem(w, 400, "请选择项目")
		return
	}
	if !h.allowed(w, r, project, "read", "") {
		return
	}
	offset, err := catalogQuery(r.URL.Query(), false)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := factory.List(r.Context(), h.backend.Namespace(), project, offset)
	if !backendError(w, err) {
		result := page[factory.Session]{Items: items}
		if len(items) == 100 {
			next := int64(offset + 100)
			result.NextOffset = &next
		}
		write(w, 200, result)
	}
}

func (h handler) factoryRead(w http.ResponseWriter, r *http.Request, action string) *factory.Session {
	if !validID(w, r) {
		return nil
	}
	s, err := factory.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if backendError(w, err) || !h.allowed(w, r, s.ProjectID, action, s.CreatedBy) {
		return nil
	}
	return s
}

func (h handler) factorySession(w http.ResponseWriter, r *http.Request) {
	s := h.factoryRead(w, r, "read")
	if s == nil {
		return
	}
	if err := scheduler.SyncFactory(r.Context(), h.backend.Namespace(), s, h.backend); backendError(w, err) {
		return
	}
	s, err := factory.Get(r.Context(), h.backend.Namespace(), s.ID)
	if !backendError(w, err) {
		write(w, 200, s)
	}
}

func (h handler) factoryMessage(w http.ResponseWriter, r *http.Request) {
	s := h.factoryRead(w, r, "manage_factory")
	if s == nil || !h.allowed(w, r, s.ProjectID, "run", "") {
		return
	}
	var in factory.Input
	if !decode(w, r, &in) {
		return
	}
	record, err := h.generateFactory(r.Context(), s, in)
	if !backendError(w, err) {
		write(w, 202, record)
	}
}

func (h handler) factoryApply(w http.ResponseWriter, r *http.Request) {
	s := h.factoryRead(w, r, "manage_factory")
	if s == nil {
		return
	}
	var in struct {
		TurnID   string `json:"turn_id"`
		Revision int    `json:"revision"`
	}
	if !decode(w, r, &in) {
		return
	}
	if _, err := uuid.Parse(in.TurnID); err != nil || in.Revision < 1 {
		problem(w, 400, "需要有效候选和修订号")
		return
	}
	out, err := factory.Apply(r.Context(), h.backend.Namespace(), s.ID, in.TurnID, in.Revision)
	if !backendError(w, err) {
		write(w, 200, out)
	}
}
