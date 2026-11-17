package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"net/http"
)

func (h handler) perforceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /projects/check", h.checkProject)
	mux.HandleFunc("PATCH /projects/{id}/workspace", h.bindPerforceDirectory)
	mux.HandleFunc("GET /runs/{id}/perforce", h.perforceDelivery)
	mux.HandleFunc("POST /runs/{id}/perforce", h.deliverPerforce)
}

func (h handler) bindPerforceDirectory(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.allowed(w, r, "", "admin", "") {
		return
	}
	var body struct {
		Source string `json:"source"`
	}
	if !decode(w, r, &body) {
		return
	}
	project, err := workspace.ProjectByID(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if project.VCS != "perforce" {
		problem(w, 400, "仅 Perforce 项目支持绑定本地工作区")
		return
	}
	previous, id := project.Source, project.ID
	project.Source, project.ID = body.Source, ""
	if err = project.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if _, err = h.backend.CheckProject(r.Context(), *project); backendError(w, err) {
		return
	}
	out, err := workspace.BindPerforceDirectory(r.Context(), h.backend.Namespace(), id, previous, body.Source)
	if !backendError(w, err) {
		write(w, 200, out)
	}
}

func (h handler) checkProject(w http.ResponseWriter, r *http.Request) {
	if !h.allowed(w, r, "", "admin", "") {
		return
	}
	var project workspace.Project
	if !decode(w, r, &project) {
		return
	}
	if err := project.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	out, err := h.backend.CheckProject(r.Context(), project)
	if !backendError(w, err) {
		write(w, 200, out)
	}
}

func (h handler) perforceDelivery(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.runAllowed(w, r, "read") {
		return
	}
	out, err := workspace.PerforceDelivery(r.Context(), r.PathValue("id"))
	if err == nil {
		err = h.maskPerforceDirectory(r.Context(), out)
	}
	if !backendError(w, err) {
		write(w, 200, out)
	}
}

func (h handler) deliverPerforce(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.runAllowed(w, r, "run") {
		return
	}
	out, err := h.backend.DeliverPerforce(r.Context(), r.PathValue("id"))
	if err == nil {
		err = h.maskPerforceDirectory(r.Context(), out)
	}
	if !backendError(w, err) {
		write(w, 202, out)
	}
}

func (h handler) maskPerforceDirectory(ctx context.Context, out *workspace.P4Delivery) error {
	if out == nil {
		return nil
	}
	admin, err := h.visible(ctx, "", "admin")
	if !admin {
		out.LocalDirectory = ""
	}
	return err
}
