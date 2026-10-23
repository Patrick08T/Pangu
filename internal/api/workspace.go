package api

import (
	"net/http"

	"github.com/Patrick08T/Pangu/internal/workspace"
)

func (h handler) registerProject(w http.ResponseWriter, r *http.Request) {
	var project workspace.Project
	if !decode(w, r, &project) {
		return
	}
	if err := project.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.allowed(w, r, "", "admin", "") {
		return
	}
	result, err := h.backend.RegisterProject(r.Context(), project)
	if !backendError(w, err) {
		write(w, 201, result)
	}
}

func (h handler) projects(w http.ResponseWriter, r *http.Request) {
	result, err := h.backend.Projects(r.Context())
	if err == nil {
		result, err = h.projectList(r.Context(), result)
	}
	if !backendError(w, err) {
		write(w, 200, result)
	}
}
