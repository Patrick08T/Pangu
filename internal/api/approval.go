package api

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"net/http"
	"strconv"
)

type approvalDetails struct {
	Approval  *approval.Gate `json:"approval"`
	RunStatus string         `json:"run_status"`
	Stopped   bool           `json:"stopped"`
}

func (h handler) approvalDetail(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	gate, err := approval.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if approvalError(w, err) || !h.allowed(w, r, gate.ProjectID, "read", gate.OwnerID) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), gate.RunID)
	if backendError(w, err) {
		return
	}
	write(w, 200, approvalDetails{gate, run.Status, run.StopConfirmed || run.CancelRequested})
}

func (h handler) approvalList(w http.ResponseWriter, r *http.Request) {
	q := approval.Query{Limit: 20}
	for key, values := range r.URL.Query() {
		if len(values) != 1 {
			problem(w, 400, "筛选参数重复")
			return
		}
		value := values[0]
		var err error
		switch key {
		case "project_id":
			q.ProjectID = value
			_, err = uuid.Parse(value)
		case "offset":
			q.Offset, err = strconv.Atoi(value)
		case "limit":
			q.Limit, err = strconv.Atoi(value)
		case "status":
			q.Status = value
			switch value {
			case "waiting", "changes_requested", "approved", "rejected", "revoked", "consumed", "expired", "invalidated":
			default:
				err = approval.ErrInvalid
			}
		case "run_id":
			q.RunID = value
			_, err = uuid.Parse(value)
		case "node_id":
			q.NodeID = value
		default:
			err = approval.ErrInvalid
		}
		if err != nil {
			problem(w, 400, "审核筛选参数无效")
			return
		}
	}
	if q.ProjectID == "" || q.Offset < 0 || q.Limit < 1 || q.Limit > 100 {
		problem(w, 400, "必须指定项目和有效分页")
		return
	}
	if !h.allowed(w, r, q.ProjectID, "read", "") {
		return
	}
	page, err := approval.Browse(r.Context(), h.backend.Namespace(), q)
	if err == nil {
		err = h.businessApprovals(r, page)
	}
	if !approvalError(w, err) {
		write(w, 200, page)
	}
}

func (h handler) approvals(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if backendError(w, err) {
		return
	}
	if !h.allowed(w, r, run.ProjectID, "read", run.CreatedBy) {
		return
	}
	items, err := approval.List(r.Context(), h.backend.Namespace(), run.ID)
	if !approvalError(w, err) {
		write(w, 200, items)
	}
}

func (h handler) decideApproval(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	gate, err := approval.Get(r.Context(), h.backend.Namespace(), r.PathValue("id"))
	if approvalError(w, err) {
		return
	}
	if !h.allowed(w, r, gate.ProjectID, "manage_run", gate.OwnerID) {
		return
	}
	run, err := h.backend.GetRun(r.Context(), gate.RunID)
	if backendError(w, err) {
		return
	}
	if run.StopConfirmed || run.Status == "cancelling" {
		problem(w, 409, "运行已停止或正在取消，不能继续审批")
		return
	}
	var input approval.Decision
	if !decode(w, r, &input) {
		return
	}
	item, err := approval.Decide(r.Context(), identity.Current(r.Context()), gate.ID, input)
	if !approvalError(w, err) {
		write(w, 200, item)
	}
}

func approvalError(w http.ResponseWriter, err error) bool {
	for _, item := range []struct {
		err    error
		status int
	}{{approval.ErrNotFound, 404}, {approval.ErrConflict, 409}, {approval.ErrExpired, 410}, {approval.ErrInvalid, 400}} {
		if errors.Is(err, item.err) {
			problem(w, item.status, item.err.Error())
			return true
		}
	}
	return backendError(w, err)
}
