package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Patrick08T/Pangu/internal/artifact"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/jackc/pgx/v5"
)

type feedbackState struct {
	SubjectHash string               `json:"subject_hash"`
	CanAccept   bool                 `json:"can_accept"`
	Feedback    *evaluation.Feedback `json:"feedback"`
}

func (h handler) evaluationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /runs/{id}/feedback", h.feedback)
	mux.HandleFunc("PUT /runs/{id}/feedback", h.feedback)
	mux.HandleFunc("GET /projects/{id}/quality", h.quality)
	mux.HandleFunc("GET /evaluations/cases", h.evaluationCases)
	mux.HandleFunc("POST /evaluations/cases", h.evaluationCases)
	mux.HandleFunc("GET /evaluations/cases/{id}", h.evaluationCase)
	mux.HandleFunc("DELETE /evaluations/cases/{id}", h.evaluationCase)
	mux.HandleFunc("GET /evaluations/cases/{id}/runs", h.evaluationRuns)
	mux.HandleFunc("POST /evaluations/cases/{id}/runs", h.evaluationRuns)
}

func (h handler) feedback(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	action := "read"
	if r.Method == "PUT" {
		action = "manage_run"
	}
	if !h.allowed(w, r, run.ProjectID, action, run.CreatedBy) {
		return
	}
	state, err := h.feedbackState(r, run)
	if evaluationError(w, err) {
		return
	}
	if r.Method == "GET" {
		write(w, 200, state)
		return
	}
	if actor := identity.Current(r.Context()); actor.ID == "" || actor.TokenID != "" {
		problem(w, 403, "人工反馈需要飞书会话，API Token 不能代替人工验收")
		return
	}
	var input evaluation.FeedbackInput
	if !decode(w, r, &input) {
		return
	}
	if err = input.Validate(); evaluationError(w, err) {
		return
	}
	if state.SubjectHash == "" || state.SubjectHash != input.SubjectHash || input.Decision == "accepted" && !state.CanAccept {
		problem(w, 409, "运行尚未成功结束、产物已过期或摘要变化，请重新读取后评定")
		return
	}
	out, err := evaluation.SaveFeedback(r.Context(), h.backend.Namespace(), run.ProjectID, run.ID, identity.Current(r.Context()), input)
	if !evaluationError(w, err) {
		write(w, 200, out)
	}
}

func (h handler) feedbackState(r *http.Request, run *scheduler.RunDetails) (*feedbackState, error) {
	out := &feedbackState{}
	feedback, err := evaluation.FeedbackForRun(r.Context(), h.backend.Namespace(), run.ID)
	if err != nil && !errors.Is(err, evaluation.ErrNotFound) {
		return nil, err
	}
	out.Feedback = feedback
	if !run.StopConfirmed || run.LogsExpired {
		return out, nil
	}
	files, err := artifact.List(r.Context(), run.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	value := struct {
		Run        string
		Status     string
		Definition any
		Outputs    []scheduler.NodeRun
		Files      *artifact.Manifest
	}{run.ID, run.Status, run.Definition, run.Nodes, files}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	out.SubjectHash = hex.EncodeToString(hash[:])
	out.CanAccept = run.Status == "succeeded"
	return out, nil
}

func (h handler) quality(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) || !h.allowed(w, r, r.PathValue("id"), "read", "") {
		return
	}
	out, err := evaluation.ProjectQuality(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if !evaluationError(w, err) {
		write(w, 200, out)
	}
}

func evaluationError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{{evaluation.ErrNotFound, 404}, {evaluation.ErrInvalid, 400}, {evaluation.ErrConflict, 409}, {evaluation.ErrExpired, 410}} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return backendError(w, err)
}
