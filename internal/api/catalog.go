package api

import (
	"net/http"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
)

func (h handler) catalogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /models/codex", h.codexModels)
	mux.HandleFunc("POST /catalog/{id}/draft/preflight", h.draftPreflight)
	mux.HandleFunc("POST /catalog/{id}/trials", h.agentTrials)
	mux.HandleFunc("GET /catalog/{id}/trials", h.agentTrials)
	mux.HandleFunc("GET /runnables", h.runnableChoices)
	mux.HandleFunc("GET /connections/choices", h.connectionChoices)
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
	mux.HandleFunc("POST /versions/{id}/run", h.runVersion)
	mux.HandleFunc("POST /versions/{id}/preflight", h.versionPreflight)
}

func (h handler) listCatalog(w http.ResponseWriter, r *http.Request) {
	if !h.allowed(w, r, "", "read_catalog", "") {
		return
	}
	offset, err := catalogQuery(r.URL.Query(), false)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := catalog.List(r.Context(), h.backend.Namespace(), r.URL.Query().Get("kind"), offset)
	if backendError(w, err) {
		return
	}
	result := page[catalog.Object]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		result.NextOffset = &next
	}
	write(w, 200, result)
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
	if !h.allowed(w, r, "", catalogAction("configure"), "") {
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

func (h handler) runVersion(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, true, "read") {
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
	key := r.Header.Get("Idempotency-Key")
	if len(r.Header.Values("Idempotency-Key")) != 1 || !submission.PublicKey(key) {
		problem(w, 400, "需要有效且唯一的 Idempotency-Key")
		return
	}
	item, err := h.versionSubmission(r.Context(), r.PathValue("id"), input.ProjectID, input.Input, key)
	if !backendError(w, err) {
		w.Header().Set("Location", "/api/requests/"+item.ID)
		write(w, 202, item)
	}
}
