package api

import (
	"net/http"
	"time"

	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/google/uuid"
)

type memoryObservationView struct {
	ID               string    `json:"id"`
	VersionID        string    `json:"version_id"`
	RunID            string    `json:"run_id"`
	DeliveryRevision int64     `json:"delivery_revision"`
	FinishedAt       time.Time `json:"finished_at"`
}

func (h handler) experienceObserve(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "configure")
	if !ok || !experienceVersionID(w, r) {
		return
	}
	var input struct {
		RunID string `json:"run_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	if _, err := uuid.Parse(input.RunID); err != nil {
		problem(w, 400, "实际运行 ID 无效")
		return
	}
	result, err := learning.ObserveMemory(r.Context(), scope, r.PathValue("version"), input.RunID)
	if experienceError(w, err) {
		return
	}
	write(w, 201, memoryObservationView{result.ID, result.VersionID, result.RunID, result.DeliveryRevision, result.FinishedAt})
}
