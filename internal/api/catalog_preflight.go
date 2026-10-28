package api

import (
	"net/http"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (h handler) versionPreflight(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.catalogAllowed(w, r, true, "read") {
		return
	}
	var input struct {
		Input     string `json:"input"`
		ProjectID string `json:"project_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !h.allowed(w, r, input.ProjectID, "run", "") {
		return
	}
	version, err := catalog.ReadVersion(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if version.Archived {
		backendError(w, catalog.ErrArchived)
		return
	}
	var definition *workflow.Definition
	if version.Kind == "connection" {
		definition = &workflow.Definition{ProjectID: input.ProjectID, Nodes: []workflow.Node{{ID: "connection-check", Executor: "eino", Instruction: "检查配置", ConnectionVersion: version.ID, Model: version.Spec.Connection.Model}}}
	} else {
		definition, err = catalog.Compile(r.Context(), h.backend.Namespace(), version.ID, input.ProjectID, input.Input)
		if backendError(w, err) {
			return
		}
	}
	h.inspectWorkflow(w, r, *definition)
}
