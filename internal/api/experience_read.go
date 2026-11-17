package api

import (
	"net/http"
	"strconv"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/google/uuid"
)

func experiencePage(w http.ResponseWriter, r *http.Request) (experience.Page, bool) {
	page := experience.Page{AfterID: r.URL.Query().Get("cursor"), Limit: 50}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			problem(w, 400, "分页 limit 必须为 1..100")
			return page, false
		}
		page.Limit = n
	}
	return page, true
}

func (h handler) experienceList(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok {
		return
	}
	page, ok := experiencePage(w, r)
	if !ok {
		return
	}
	result, err := learning.NewStore().ListVersions(r.Context(), scope, page)
	if !experienceError(w, err) {
		write(w, 200, result)
	}
}

func experienceVersionID(w http.ResponseWriter, r *http.Request) bool {
	if _, err := uuid.Parse(r.PathValue("version")); err != nil {
		problem(w, 400, "无效的经验版本 ID")
		return false
	}
	return true
}

func (h handler) experienceDetail(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok || !experienceVersionID(w, r) {
		return
	}
	result, err := learning.NewStore().Read(r.Context(), scope, r.PathValue("version"))
	if !experienceError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) experienceUsages(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok || !experienceVersionID(w, r) {
		return
	}
	page, ok := experiencePage(w, r)
	if !ok {
		return
	}
	result, err := learning.NewStore().ListUsages(r.Context(), scope, r.PathValue("version"), page)
	if !experienceError(w, err) {
		write(w, 200, result)
	}
}

func (h handler) experienceCapabilities(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.experienceScope(w, r, "read")
	if !ok {
		return
	}
	manage, err := experienceGovernanceAllowed(r.Context(), scope)
	if backendError(w, err) {
		return
	}
	submit, err := h.visible(r.Context(), scope.ProjectID, "run")
	if backendError(w, err) {
		return
	}
	submit = submit && identity.Current(r.Context()).TokenID == ""
	_, entryReady := h.backend.(entrySubmitter)
	_, evolutionReady := h.evolutionRuntime()
	_, startReady := h.backend.(evolutionStarter)
	_, rebuildReady := h.backend.(rebuildProvider)
	_, deliveryReady := h.deliveryStore()
	write(w, 200, map[string]any{
		"permissions":       map[string]bool{"manage_policy": manage, "manage_sources": manage, "revoke": manage, "delete": manage, "rollback": manage && evolutionReady, "submit_entry": submit},
		"features":          map[string]bool{"memory_read": true, "claim_verification": true, "delivery_promotion": deliveryReady, "evolution": evolutionReady && startReady, "run_lineage": true, "approval_rebuild": rebuildReady, "entry_submission": entryReady},
		"supported_metrics": []string{"acceptance"},
	})
}
