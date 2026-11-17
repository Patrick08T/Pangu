package api

import (
	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/google/uuid"
	"net/http"
	"strings"
)

func businessPurpose(purpose string) bool { return purpose == "" || purpose == "business" }

// Ordinary run surfaces must not become an optimizer's hidden-case read API.
// Purpose comes from trusted scheduler state, never a request query or body.
func (h handler) businessSurface(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			if _, err := uuid.Parse(parts[1]); err == nil {
				switch parts[0] {
				case "runs":
					if !h.businessRunID(w, r, parts[1]) {
						return
					}
				case "approvals":
					gate, err := approval.Get(r.Context(), h.backend.Namespace(), parts[1])
					if approvalError(w, err) {
						return
					}
					if !h.businessRunID(w, r, gate.RunID) {
						return
					}
				case "requests":
					item, err := h.backend.GetRequest(r.Context(), parts[1])
					if backendError(w, err) {
						return
					}
					if item == nil || !businessPurpose(item.Snapshot.Purpose) || (item.Snapshot.Purpose == "" && len(item.Snapshot.Workflow.Nodes) == 0) {
						problem(w, 404, "运行或请求不存在")
						return
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h handler) businessApprovals(r *http.Request, page *approval.Page) error {
	items := []approval.Summary{}
	for _, item := range page.Items {
		run, err := h.backend.GetRun(r.Context(), item.RunID)
		if err != nil {
			return err
		}
		if run == nil {
			continue
		}
		visible, err := h.businessRun(r.Context(), run.RunSummary)
		if err != nil {
			return err
		}
		if visible {
			items = append(items, item)
		}
	}
	page.Items = items
	return nil
}

func (h handler) businessRunID(w http.ResponseWriter, r *http.Request, id string) bool {
	item, err := h.backend.GetRun(r.Context(), id)
	if backendError(w, err) {
		return false
	}
	if item == nil || item.ID != id {
		problem(w, 404, "运行或请求不存在")
		return false
	}
	visible, err := h.businessRun(r.Context(), item.RunSummary)
	if backendError(w, err) {
		return false
	}
	if !visible {
		problem(w, 404, "运行或请求不存在")
		return false
	}
	return true
}
