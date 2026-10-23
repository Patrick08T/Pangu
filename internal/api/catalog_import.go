package api

import (
	"net/http"

	"github.com/Patrick08T/Pangu/internal/catalog"
)

type catalogDocument struct {
	Format string `json:"format"`
	catalog.NewObject
}

func (h handler) importCatalog(w http.ResponseWriter, r *http.Request) {
	var document catalogDocument
	if !decode(w, r, &document) {
		return
	}
	if document.Format != "pangu.catalog.v1" {
		problem(w, 400, "仅接受 pangu.catalog.v1 格式")
		return
	}
	if err := document.NewObject.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.allowed(w, r, document.ProjectID, catalogAction(document.Kind, "configure"), "") {
		return
	}
	item, err := catalog.Create(r.Context(), h.backend.Namespace(), document.NewObject)
	if !backendError(w, err) {
		write(w, 201, item)
	}
}

func (h handler) exportCatalog(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.catalogAllowed(w, r, r.Pattern == "GET /versions/{id}/export", "read") {
		return
	}
	var item catalog.NewObject
	if r.Pattern == "GET /versions/{id}/export" {
		v, err := catalog.ReadVersion(r.Context(), h.backend.Namespace(), r.PathValue("id"))
		if backendError(w, err) {
			return
		}
		object, err := catalog.Get(r.Context(), h.backend.Namespace(), v.ObjectID)
		if backendError(w, err) {
			return
		}
		item = catalog.NewObject{Kind: v.Kind, ProjectID: v.ProjectID, Name: object.Name, Spec: v.Spec}
	} else {
		object, err := catalog.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
		if backendError(w, err) {
			return
		}
		item = object.NewObject
	}
	w.Header().Set("Content-Disposition", `attachment; filename="pangu-catalog.json"`)
	write(w, 200, catalogDocument{Format: "pangu.catalog.v1", NewObject: item})
}
