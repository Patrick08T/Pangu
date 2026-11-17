package api

import (
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
	"net/http"
)

func (h handler) evolutionEnvironments(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.evolutionScope(w, r)
	if !ok {
		return
	}
	if r.Method == "GET" {
		page, ok := experiencePage(w, r)
		if !ok {
			return
		}
		result, err := (evolution.Service{}).ListEnvironments(r.Context(), scope, evolution.QueryPage{AfterID: page.AfterID, Limit: page.Limit})
		if !evolutionError(w, err) {
			write(w, 200, result)
		}
		return
	}
	var input struct {
		ID                string `json:"id"`
		ExpectedRevision  *int64 `json:"expected_revision"`
		Backend           string `json:"backend"`
		ConnectionVersion string `json:"connection_version"`
		Enabled           bool   `json:"enabled"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ExpectedRevision == nil || *input.ExpectedRevision < 0 {
		problem(w, 400, "环境配置必须提供 expected_revision")
		return
	}
	if _, err := uuid.Parse(input.ConnectionVersion); err != nil {
		problem(w, 400, "无效的连接版本 ID")
		return
	}
	result, err := (evolution.Service{}).RegisterEnvironment(r.Context(), scope, evolution.Environment{ID: input.ID, ProjectID: scope.ProjectID, Backend: input.Backend, ConnectionVersion: input.ConnectionVersion, Enabled: input.Enabled}, *input.ExpectedRevision)
	if !evolutionError(w, err) {
		write(w, 201, result)
	}
}
