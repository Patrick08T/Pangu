package api

import (
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"net/http"
	"time"
)

func (h handler) identityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/memberships", h.memberships)
	mux.HandleFunc("GET /auth/tokens", h.tokens)
	mux.HandleFunc("POST /auth/tokens", h.issueToken)
	mux.HandleFunc("DELETE /auth/tokens/{id}", h.revokeToken)
	mux.HandleFunc("GET /projects/{id}/members", h.members)
	mux.HandleFunc("PUT /projects/{id}/members", h.grantMember)
	mux.HandleFunc("DELETE /projects/{id}/members/{user}", h.removeMember)
	mux.HandleFunc("POST /users/{id}/disable", h.disableUser)
	mux.HandleFunc("POST /users/{id}/enable", h.enableUser)
}

func (h handler) memberships(w http.ResponseWriter, r *http.Request) {
	items, err := identity.Memberships(r.Context(), identity.Current(r.Context()))
	if !backendError(w, err) {
		write(w, 200, items)
	}
}
func (h handler) members(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	items, err := identity.Members(r.Context(), identity.Current(r.Context()), r.PathValue("id"))
	if !backendError(w, err) {
		write(w, 200, items)
	}
}
func (h handler) grantMember(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	var input struct {
		OpenID string `json:"open_id"`
		Role   string `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	item, err := identity.Grant(r.Context(), identity.Current(r.Context()), r.PathValue("id"), input.OpenID, input.Role)
	if !backendError(w, err) {
		write(w, 200, item)
	}
}
func (h handler) removeMember(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if _, err := uuid.Parse(r.PathValue("user")); err != nil {
		problem(w, 400, "user 必须为 UUID")
		return
	}
	if !backendError(w, identity.RemoveMember(r.Context(), identity.Current(r.Context()), r.PathValue("id"), r.PathValue("user"))) {
		w.WriteHeader(204)
	}
}
func (h handler) disableUser(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !backendError(w, identity.Disable(r.Context(), identity.Current(r.Context()), r.PathValue("id"))) {
		w.WriteHeader(204)
	}
}
func (h handler) enableUser(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !backendError(w, identity.Enable(r.Context(), identity.Current(r.Context()), r.PathValue("id"))) {
		w.WriteHeader(204)
	}
}
func (h handler) tokens(w http.ResponseWriter, r *http.Request) {
	items, err := identity.Tokens(r.Context(), identity.Current(r.Context()))
	if !backendError(w, err) {
		write(w, 200, items)
	}
}
func (h handler) issueToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID string    `json:"project_id"`
		Name      string    `json:"name"`
		Scopes    []string  `json:"scopes"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if !decode(w, r, &input) {
		return
	}
	item, raw, err := identity.IssueToken(r.Context(), identity.Current(r.Context()), input.ProjectID, input.Name, input.Scopes, input.ExpiresAt)
	if !backendError(w, err) {
		write(w, 201, struct {
			identity.Token
			Value string `json:"value"`
		}{item, raw})
	}
}
func (h handler) revokeToken(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !backendError(w, identity.RevokeToken(r.Context(), identity.Current(r.Context()), r.PathValue("id"))) {
		w.WriteHeader(204)
	}
}
