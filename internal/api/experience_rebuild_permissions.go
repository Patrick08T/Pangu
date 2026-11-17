package api

import (
	"errors"
	"net/http"

	"github.com/Patrick08T/Pangu/internal/identity"
)

// This is conservative UI eligibility, not an authorization or clean-baseline receipt.
// The command rechecks the original request, current generations and running attempts.
func (h handler) rebuildAvailability(r *http.Request) (bool, string, error) {
	if _, ok := h.backend.(rebuildProvider); !ok {
		return false, "失效运行重建尚未接通", nil
	}
	run, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		return false, "", err
	}
	if run == nil {
		return false, "无法核实原始运行", nil
	}
	actor := identity.Current(r.Context())
	if actor.ID == "" || actor.Namespace != h.backend.Namespace() || actor.ID != run.CreatedBy || actor.TokenID != "" {
		return false, "仅原创建人的交互会话可重建", nil
	}
	for _, action := range []string{"run", "manage_run"} {
		err = h.access.Authorize(r.Context(), run.ProjectID, action, run.CreatedBy)
		if errors.Is(err, identity.ErrForbidden) {
			return false, "当前缺少运行或运行管理权限", nil
		}
		if err != nil {
			return false, "", err
		}
	}
	if !run.StopConfirmed {
		return false, "等待运行停止确认", nil
	}
	return true, "", nil
}
