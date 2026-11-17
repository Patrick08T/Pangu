package api

import (
	"errors"
	"net/http"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
)

func (h handler) experienceRoutes(mux *http.ServeMux) {
	h.evolutionRoutes(mux)
	mux.HandleFunc("POST /projects/{id}/entries/{entry}/runs", h.experienceEntry)
	mux.HandleFunc("GET /runs/{id}/experience", h.experienceRunLineage)
	mux.HandleFunc("GET /projects/{id}/experience-artifacts/{artifact}", h.experienceArtifact)
	mux.HandleFunc("POST /projects/{id}/experiences/{version}/observe", h.experienceObserve)
	mux.HandleFunc("GET /runs/{id}/rebuild-intents", h.experienceRebuilds)
	mux.HandleFunc("POST /runs/{id}/rebuild-intents/{intent}", h.experienceRebuild)
	mux.HandleFunc("GET /projects/{id}/experience-policy", h.experiencePolicy)
	mux.HandleFunc("PUT /projects/{id}/experience-policy", h.experiencePolicy)
	mux.HandleFunc("GET /projects/{id}/experience-capabilities", h.experienceCapabilities)
	mux.HandleFunc("GET /projects/{id}/experiences", h.experienceList)
	mux.HandleFunc("GET /projects/{id}/experiences/{version}", h.experienceDetail)
	mux.HandleFunc("GET /projects/{id}/experiences/{version}/usages", h.experienceUsages)
	mux.HandleFunc("POST /projects/{id}/experiences/{version}/revoke", h.experienceRevoke)
	mux.HandleFunc("DELETE /projects/{id}/experiences/{version}", h.experienceRevoke)
	mux.HandleFunc("POST /projects/{id}/experiences/{version}/activate", h.experienceActivate)
	mux.HandleFunc("POST /projects/{id}/experiences/{version}/delivery", h.experienceDelivery)
	mux.HandleFunc("POST /projects/{id}/experience-sources", h.experienceSource)
	mux.HandleFunc("DELETE /projects/{id}/experience-sources/{source}", h.experienceSource)
	mux.HandleFunc("GET /projects/{id}/experience-checkers", h.experienceCheckers)
	mux.HandleFunc("POST /projects/{id}/experience-checkers", h.experienceCheckers)
}

func (h handler) experienceScope(w http.ResponseWriter, r *http.Request, action string) (experience.Scope, bool) {
	if !validID(w, r) {
		return experience.Scope{}, false
	}
	actor := identity.Current(r.Context())
	if actor.ID == "" || actor.Namespace != h.backend.Namespace() {
		problem(w, 403, "经验操作需要当前项目身份")
		return experience.Scope{}, false
	}
	if action != "read" && actor.TokenID != "" {
		problem(w, 403, "经验治理需要人工会话，API Token 不可代替授权")
		return experience.Scope{}, false
	}
	if !h.allowed(w, r, r.PathValue("id"), action, "") {
		return experience.Scope{}, false
	}
	return experience.Scope{Namespace: h.backend.Namespace(), ProjectID: r.PathValue("id"), ActorID: actor.ID}, true
}

func experienceError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{
		{experience.ErrInvalid, 400}, {experience.ErrForbidden, 403}, {experience.ErrNotFound, 404},
		{experience.ErrConflict, 409}, {experience.ErrInvalidated, 410},
	} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return evaluationError(w, err)
}
