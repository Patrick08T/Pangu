package api

import (
	"github.com/Patrick08T/Pangu/internal/catalog"
	"net/http"
)

func (h handler) runnableChoices(w http.ResponseWriter, r *http.Request) {
	for key := range r.URL.Query() {
		if key != "offset" {
			problem(w, 400, "只接受 offset")
			return
		}
	}
	offset, err := catalogQuery(r.URL.Query(), false)
	if err != nil {
		problem(w, 400, "必须提供非负 offset")
		return
	}
	if !h.allowed(w, r, "", "read_catalog", "") {
		return
	}
	items, err := catalog.RunnableChoices(r.Context(), h.backend.Namespace(), offset)
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

func (h handler) connectionChoices(w http.ResponseWriter, r *http.Request) {
	for key := range r.URL.Query() {
		if key != "offset" {
			problem(w, 400, "只接受 offset")
			return
		}
	}
	offset, err := catalogQuery(r.URL.Query(), false)
	if err != nil {
		problem(w, 400, "必须提供非负 offset")
		return
	}
	if !h.allowed(w, r, "", "read_catalog", "") {
		return
	}
	items, err := catalog.ConnectionChoices(r.Context(), h.backend.Namespace(), offset)
	if backendError(w, err) {
		return
	}
	out := page[catalog.ConnectionChoice]{Items: items}
	if len(items) == 100 {
		next := int64(offset + 100)
		out.NextOffset = &next
	}
	write(w, 200, out)
}
