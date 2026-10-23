package api

import (
	"net/http"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (h handler) validate(w http.ResponseWriter, r *http.Request) {
	var definition workflow.Definition
	if !decode(w, r, &definition) {
		return
	}
	layers, err := definition.Layers()
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	ids := make([][]string, len(layers))
	for i, layer := range layers {
		ids[i] = make([]string, len(layer))
		for j, node := range layer {
			ids[i][j] = node.ID
		}
	}
	write(w, 200, struct {
		Valid  bool       `json:"valid"`
		Layers [][]string `json:"layers"`
	}{Valid: true, Layers: ids})
}

func (h handler) preflight(w http.ResponseWriter, r *http.Request) {
	var definition workflow.Definition
	if !decode(w, r, &definition) {
		return
	}
	if _, err := definition.Layers(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.allowed(w, r, definition.ProjectID, "run", "") {
		return
	}
	h.inspectWorkflow(w, r, definition)
}

func (h handler) inspectWorkflow(w http.ResponseWriter, r *http.Request, definition workflow.Definition) {
	result, err := h.backend.Preflight(r.Context(), definition)
	if !backendError(w, err) {
		status := http.StatusOK
		if !result.Ready {
			status = http.StatusServiceUnavailable
		}
		write(w, status, result)
	}
}
