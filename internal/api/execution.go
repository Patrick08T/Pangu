package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h handler) attempts(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	offset, limit, err := execution.ParsePage(r.URL.Query())
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	if !h.runAllowed(w, r, "read") || !logsAvailable(w, r) {
		return
	}
	page, err := execution.ListAttempts(r.Context(), r.PathValue("id"), offset, limit)
	if !backendError(w, err) {
		write(w, 200, page)
	}
}

func (h handler) attempt(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	number, err := strconv.Atoi(r.PathValue("attempt"))
	if _, idErr := uuid.Parse(r.PathValue("node")); idErr != nil || err != nil || number < 1 || len(r.URL.Query()) > 0 {
		problem(w, 400, "节点运行 ID 或尝试编号无效")
		return
	}
	if !h.runAllowed(w, r, "read") || !logsAvailable(w, r) {
		return
	}
	item, err := execution.AttemptByID(r.Context(), r.PathValue("id"), r.PathValue("node"), number)
	if errors.Is(err, pgx.ErrNoRows) {
		problem(w, 404, "节点尝试不存在")
		return
	}
	if !backendError(w, err) {
		write(w, 200, item)
	}
}

func logsAvailable(w http.ResponseWriter, r *http.Request) bool {
	expired, err := workspace.Expired(r.Context(), r.PathValue("id"))
	if expired {
		err = workspace.ErrExpired
	}
	return !backendError(w, err)
}
