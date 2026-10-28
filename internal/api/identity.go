package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/identity"
)

type Access interface {
	Handler(http.Handler) http.Handler
	Authorize(context.Context, string, string, string) error
}

func (h handler) allowed(w http.ResponseWriter, r *http.Request, project, action, owner string) bool {
	return !backendError(w, h.access.Authorize(r.Context(), project, action, owner))
}

func (h handler) runAllowed(w http.ResponseWriter, r *http.Request, action string) bool {
	item, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	return !backendError(w, err) && h.allowed(w, r, item.ProjectID, action, item.CreatedBy)
}

func (h handler) cronAllowed(w http.ResponseWriter, r *http.Request, action string) bool {
	item, err := h.backend.GetCron(r.Context(), r.PathValue("id"))
	return !backendError(w, err) && h.allowed(w, r, item.Workflow.ProjectID, action, item.CreatedBy)
}

func catalogAction(kind, action string) string {
	if kind == "template" {
		if action == "read" {
			return "read_templates"
		}
		return "manage_templates"
	}
	if kind == "connection" {
		return "manage_connections"
	}
	return action
}

func (h handler) catalogAllowed(w http.ResponseWriter, r *http.Request, version bool, action string) bool {
	var project, kind string
	if version {
		item, err := catalog.ReadVersion(r.Context(), h.backend.Namespace(), r.PathValue("id"))
		if backendError(w, err) {
			return false
		}
		project, kind = item.ProjectID, item.Kind
	} else {
		item, err := catalog.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
		if backendError(w, err) {
			return false
		}
		project, kind = item.ProjectID, item.Kind
	}
	return h.allowed(w, r, project, catalogAction(kind, action), "")
}

func (h handler) visible(ctx context.Context, project, action string) (bool, error) {
	err := h.access.Authorize(ctx, project, action, "")
	if errors.Is(err, identity.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}
