package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Patrick08T/Pangu/internal/learning"
)

type experienceCheckerView struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	SourceKind string    `json:"source_kind"`
	NodeID     string    `json:"node_id"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func checkerView(c learning.Checker) experienceCheckerView {
	return experienceCheckerView{c.ID, c.Spec.Kind, c.Spec.SourceKind, c.Spec.NodeID, c.CreatedAt, c.ExpiresAt}
}

func (h handler) experienceCheckers(w http.ResponseWriter, r *http.Request) {
	action := "read"
	if r.Method == "POST" {
		action = "configure"
	}
	scope, ok := h.experienceScope(w, r, action)
	if !ok {
		return
	}
	if r.Method == "POST" {
		var input struct {
			Spec      learning.CheckerSpec `json:"spec"`
			ExpiresAt time.Time            `json:"expires_at"`
		}
		if !decode(w, r, &input) {
			return
		}
		checker, err := learning.RegisterChecker(r.Context(), scope, input.Spec, input.ExpiresAt)
		if !experienceError(w, err) {
			write(w, 201, checkerView(checker))
		}
		return
	}
	page, ok := experiencePage(w, r)
	if !ok {
		return
	}
	offset := 0
	if page.AfterID != "" {
		n, err := strconv.Atoi(page.AfterID)
		if err != nil || n < 0 || n > 10000 {
			problem(w, 400, "无效的检查器分页游标")
			return
		}
		offset = n
	}
	items, err := learning.ListCheckers(r.Context(), scope, page.Limit, offset)
	if experienceError(w, err) {
		return
	}
	views := []experienceCheckerView{}
	for _, item := range items {
		views = append(views, checkerView(item))
	}
	result := struct {
		Items      []experienceCheckerView `json:"items"`
		NextCursor string                  `json:"next_cursor,omitempty"`
	}{Items: views}
	if len(items) == page.Limit && offset+page.Limit <= 10000 {
		result.NextCursor = strconv.Itoa(offset + page.Limit)
	}
	write(w, 200, result)
}
