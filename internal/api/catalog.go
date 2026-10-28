package api

import (
	"net/http"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
)

func (h handler) catalogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /catalog", h.listCatalog)
	mux.HandleFunc("POST /catalog", h.createCatalog)
	mux.HandleFunc("POST /catalog/import", h.importCatalog)
	mux.HandleFunc("GET /catalog/{id}/export", h.exportCatalog)
	mux.HandleFunc("GET /versions/{id}/export", h.exportCatalog)
	mux.HandleFunc("GET /catalog/{id}", h.getCatalog)
	mux.HandleFunc("PUT /catalog/{id}/draft", h.saveCatalog)
	mux.HandleFunc("POST /catalog/{id}/publish", h.publishCatalog)
	mux.HandleFunc("POST /catalog/{id}/archive", h.archiveCatalog)
	mux.HandleFunc("GET /catalog/{id}/versions", h.catalogVersions)
	mux.HandleFunc("GET /versions/{id}", h.catalogVersion)
	mux.HandleFunc("POST /versions/{id}/bind", h.bindCatalog)
	mux.HandleFunc("POST /versions/{id}/run", h.runVersion)
	mux.HandleFunc("POST /versions/{id}/preflight", h.versionPreflight)
}

func (h handler) listCatalog(w http.ResponseWriter, r *http.Request) {
	offset, err := catalogQuery(r.URL.Query(), false)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := catalog.List(r.Context(), h.backend.Namespace(), r.URL.Query().Get("kind"), r.URL.Query().Get("project_id"), offset)
	if backendError(w, err) {
		return
	}
	result, err := h.catalogPage(r.Context(), items, offset)
	if !backendError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) createCatalog(w http.ResponseWriter, r *http.Request) {
	var input catalog.NewObject
	if !decode(w, r, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.allowed(w, r, input.ProjectID, catalogAction(input.Kind, "configure"), "") {
		return
	}
	item, err := catalog.Create(r.Context(), h.backend.Namespace(), input)
	if !backendError(w, err) {
		write(w, 201, item)
	}
}

func (h handler) getCatalog(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, false, "read") {
		return
	}
	item, err := catalog.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if !backendError(w, err) {
		write(w, 200, item)
	}
}

func (h handler) saveCatalog(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, false, "configure") {
		return
	}
	var input struct {
		Revision int          `json:"revision"`
		Name     string       `json:"name"`
		Spec     catalog.Spec `json:"spec"`
	}
	if !decode(w, r, &input) {
		return
	}
	item, err := catalog.Save(r.Context(), h.backend.Namespace(), r.PathValue("id"), input.Revision, input.Name, input.Spec)
	if !backendError(w, err) {
		write(w, 200, item)
	}
}

func (h handler) publishCatalog(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, false, "configure") {
		return
	}
	var input struct {
		Revision int `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	item, err := catalog.Publish(r.Context(), h.backend.Namespace(), r.PathValue("id"), input.Revision)
	if !backendError(w, err) {
		write(w, 200, item)
	}
}

func (h handler) archiveCatalog(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	var input struct {
		Archived *bool `json:"archived"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Archived == nil {
		problem(w, 400, "必须明确提供 archived")
		return
	}
	if !h.catalogAllowed(w, r, false, "configure") {
		return
	}
	if !backendError(w, catalog.Archive(r.Context(), h.backend.Namespace(), r.PathValue("id"), *input.Archived)) {
		w.WriteHeader(204)
	}
}

func (h handler) catalogVersions(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	offset, err := catalogQuery(r.URL.Query(), true)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.catalogAllowed(w, r, false, "read") {
		return
	}
	items, err := catalog.Versions(r.Context(), h.backend.Namespace(), r.PathValue("id"), offset)
	if !backendError(w, err) {
		write(w, 200, items)
	}
}

func (h handler) catalogVersion(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, true, "read") {
		return
	}
	item, err := catalog.ReadVersion(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if !backendError(w, err) {
		write(w, 200, item)
	}
}

func (h handler) bindCatalog(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, true, "read") {
		return
	}
	var input struct {
		ProjectID         string `json:"project_id"`
		Name              string `json:"name"`
		ConnectionVersion string `json:"connection_version"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !h.allowed(w, r, input.ProjectID, "configure", "") {
		return
	}
	item, err := catalog.Bind(r.Context(), h.backend.Namespace(), r.PathValue("id"), input.ProjectID, input.Name, input.ConnectionVersion)
	if !backendError(w, err) {
		write(w, 201, item)
	}
}

func (h handler) runVersion(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, true, "run") {
		return
	}
	var input struct {
		Input string `json:"input"`
	}
	if !decode(w, r, &input) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.ValidKey(key) {
		problem(w, 400, "需要有效且唯一的 Idempotency-Key")
		return
	}
	item, err := h.versionSubmission(r.Context(), r.PathValue("id"), input.Input, key)
	if !backendError(w, err) {
		w.Header().Set("Location", "/api/requests/"+item.ID)
		write(w, 202, item)
	}
}
