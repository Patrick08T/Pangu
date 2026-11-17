package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/google/uuid"
)

type evolutionProvider interface{ EvolutionService() evolution.Service }
type rollbackService interface {
	RequestRollback(context.Context, evolution.Scope, string, int, string) (*evolution.Intent, error)
}

func (h handler) evolutionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /projects/{id}/evolution/intents", h.evolutionIntents)
	mux.HandleFunc("POST /projects/{id}/evolution/intents", h.evolutionCandidate)
	mux.HandleFunc("POST /projects/{id}/evolution/intents/{intent}/decision", h.evolutionAction)
	mux.HandleFunc("POST /projects/{id}/evolution/intents/{intent}/promote", h.evolutionAction)
	mux.HandleFunc("POST /projects/{id}/evolution/intents/{intent}/observe", h.evolutionAction)
	mux.HandleFunc("GET /projects/{id}/evolution/intents/{intent}", h.evolutionIntent)
	mux.HandleFunc("POST /projects/{id}/evolution/intents/{intent}/rollback", h.evolutionRollback)
	mux.HandleFunc("GET /projects/{id}/evolution/environments", h.evolutionEnvironments)
	mux.HandleFunc("POST /projects/{id}/evolution/environments", h.evolutionEnvironments)
}

func (h handler) evolutionScope(w http.ResponseWriter, r *http.Request) (evolution.Scope, bool) {
	scope, ok := h.experienceScope(w, r, "configure")
	return evolution.Scope{Namespace: scope.Namespace, ProjectID: scope.ProjectID, ActorID: scope.ActorID}, ok
}

func (h handler) evolutionIntents(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.evolutionScope(w, r)
	if !ok {
		return
	}
	page, ok := experiencePage(w, r)
	if !ok {
		return
	}
	result, err := (evolution.Service{}).ListIntents(r.Context(), scope, evolution.QueryPage{AfterID: page.AfterID, Limit: page.Limit})
	if !evolutionError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) evolutionIntent(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.evolutionScope(w, r)
	if !ok {
		return
	}
	result, err := (evolution.Service{}).ReadIntentSummary(r.Context(), scope, r.PathValue("intent"))
	if !evolutionError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) evolutionRuntime() (evolution.Service, bool) {
	provider, ok := h.backend.(evolutionProvider)
	if !ok {
		return evolution.Service{}, false
	}
	service := provider.EvolutionService()
	return service, service.Entries != nil && service.Evidence != nil && service.Memory.Authorize != nil
}

func (h handler) evolutionRollback(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.evolutionScope(w, r)
	if !ok {
		return
	}
	var input struct {
		Generation int    `json:"generation"`
		Reason     string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	if _, err := uuid.Parse(r.PathValue("intent")); err != nil || input.Generation < 1 || strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 1024 {
		problem(w, 400, "回退意图、代际或原因无效")
		return
	}
	service, ready := h.evolutionRuntime()
	rollback, implemented := any(&service).(rollbackService)
	if !ready || !implemented {
		problem(w, 503, "进化回退执行器尚未接通")
		return
	}
	_, err := rollback.RequestRollback(r.Context(), scope, r.PathValue("intent"), input.Generation, input.Reason)
	if evolutionError(w, err) {
		return
	}
	// Return a safe query view, never a raw experiment or machine-result object.
	result, err := service.ReadIntentSummary(r.Context(), scope, r.PathValue("intent"))
	if !evolutionError(w, err) {
		write(w, 202, result)
	}
}

func evolutionError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{
		{evolution.ErrInvalid, 400}, {evolution.ErrForbidden, 403}, {evolution.ErrNotFound, 404}, {evolution.ErrConflict, 409},
		{evolution.ErrIsolation, 422}, {evolution.ErrChecker, 422}, {evolution.ErrBudget, 409},
	} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return experienceError(w, err)
}
