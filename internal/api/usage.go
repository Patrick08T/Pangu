package api

import (
	"github.com/Patrick08T/Pangu/internal/usage"
	"net/http"
)

func (h handler) usage(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.runAllowed(w, r, "read") {
		return
	}
	offset, err := catalogQuery(r.URL.Query(), true)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	result, err := usage.ForRun(r.Context(), r.PathValue("id"), offset)
	if !backendError(w, err) {
		write(w, 200, result)
	}
}
