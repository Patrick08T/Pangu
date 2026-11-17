package api

import (
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/google/uuid"
	"net/http"
)

func (h handler) scopedCatalogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /projects/{id}/runnables", h.scopedRunnables)
	mux.HandleFunc("GET /projects/{id}/versions/{version}", h.scopedVersion)
	mux.HandleFunc("GET /projects/{id}/catalog/{object}/versions", h.scopedVersions)
}

func (h handler) scopedRunnables(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok {
		return
	}
	for key := range r.URL.Query() {
		if key != "offset" {
			problem(w, 400, "只接受 offset")
			return
		}
	}
	offset, err := catalogQuery(r.URL.Query(), false)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := catalog.RunnableChoicesForProject(r.Context(), scope.Namespace, scope.ProjectID, offset)
	if backendError(w, err) {
		return
	}
	out := page[catalog.RunnableChoice]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		out.NextOffset = &next
	}
	write(w, 200, out)
}

func (h handler) scopedVersion(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok || !experienceVersionID(w, r) {
		return
	}
	if len(r.URL.Query()) != 0 {
		problem(w, 400, "版本读取不接受额外授权参数")
		return
	}
	item, err := catalog.ReadVersionScoped(r.Context(), scope.Namespace, r.PathValue("version"), catalog.VersionScope{ProjectID: scope.ProjectID})
	if !backendError(w, err) {
		write(w, 200, item)
	}
}

func (h handler) scopedVersions(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok {
		return
	}
	if _, err := uuid.Parse(r.PathValue("object")); err != nil {
		problem(w, 400, "无效目录 ID")
		return
	}
	offset, err := catalogQuery(r.URL.Query(), true)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	items, err := catalog.VersionsScoped(r.Context(), scope.Namespace, r.PathValue("object"), offset, catalog.VersionScope{ProjectID: scope.ProjectID})
	if !backendError(w, err) {
		write(w, 200, items)
	}
}
