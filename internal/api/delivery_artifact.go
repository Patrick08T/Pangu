package api

import (
	"github.com/Patrick08T/Pangu/internal/artifact"
	"github.com/Patrick08T/Pangu/internal/delivery"
	"github.com/Patrick08T/Pangu/internal/submission"
	"net/http"
)

func (h handler) exportDelivery(w http.ResponseWriter, r *http.Request) {
	item := h.deliveryAllowed(w, r, "read")
	if item == nil {
		return
	}
	if item.RequestID == "" {
		problem(w, 409, "草案尚未执行")
		return
	}
	receipt, err := submission.Get(r.Context(), h.backend.Namespace(), item.RequestID)
	if backendError(w, err) {
		return
	}
	if receipt.RunID == "" {
		problem(w, 409, "运行尚未开始")
		return
	}
	run, err := h.backend.GetRun(r.Context(), receipt.RunID)
	if backendError(w, err) {
		return
	}
	state, err := h.feedbackState(r, run)
	if evaluationError(w, err) {
		return
	}
	verified, err := delivery.Verified(r.Context(), h.backend.Namespace(), item.ID)
	if backendError(w, err) {
		return
	}
	if !verified || !state.CanAccept || state.Feedback == nil || state.Feedback.Source != "feishu_session" || state.Feedback.Decision != "accepted" || state.Feedback.SubjectHash != state.SubjectHash {
		problem(w, 409, "需要实施与只读审查的成功验证，以及针对当前产物的人工采纳")
		return
	}
	data, err := artifact.Export(r.Context(), receipt.RunID)
	if artifactError(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="pangu-delivery.zip"`)
	_, _ = w.Write(data)
}
