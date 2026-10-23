package api

import (
	"context"
	"net/http"
	"time"
)

func (h handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	checks := map[string]string{"database": "ok", "hatchet_registration": "ok"}
	status := http.StatusOK
	if h.checkDatabase(ctx) != nil {
		checks["database"], status = "unavailable", http.StatusServiceUnavailable
	}
	if h.backend.Check(ctx) != nil {
		checks["hatchet_registration"], status = "unavailable", http.StatusServiceUnavailable
	}
	state := "ready"
	if status != http.StatusOK {
		state = "not_ready"
	}
	write(w, status, struct {
		Status string            `json:"status"`
		Scope  string            `json:"scope"`
		Checks map[string]string `json:"checks"`
	}{Status: state, Scope: "api_dependencies", Checks: checks})
}
