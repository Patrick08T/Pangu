package approval

import (
	"github.com/Patrick08T/Pangu/internal/workflow"
	"slices"
	"strings"
	"unicode/utf8"
)

func validateGateDecision(gate *Gate, d Decision) error {
	if gate.Purpose == "evidence" && d.Decision == "approved" {
		return ErrInvalid
	}
	if gate.Purpose == "evidence" && d.Decision == "changes_requested" {
		hasMaterial := strings.TrimSpace(d.Reason) != ""
		for _, answer := range d.Items {
			hasMaterial = hasMaterial || strings.TrimSpace(answer.Correction) != ""
		}
		if !hasMaterial {
			return ErrInvalid
		}
	}
	return validateAnswers(gate.Review, d)
}

func sameAnswers(a, b []workflow.ReviewAnswer) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	compare := func(a, b workflow.ReviewAnswer) int { return strings.Compare(a.ItemID, b.ItemID) }
	slices.SortFunc(a, compare)
	slices.SortFunc(b, compare)
	return slices.EqualFunc(a, b, func(a, b workflow.ReviewAnswer) bool {
		left, right := slices.Clone(a.SelectedOptionIDs), slices.Clone(b.SelectedOptionIDs)
		slices.Sort(left)
		slices.Sort(right)
		return a.ItemID == b.ItemID && a.Decision == b.Decision && a.Correction == b.Correction && slices.Equal(left, right)
	})
}

func validateAnswers(review *workflow.Review, d Decision) error {
	if review == nil {
		return ErrInvalid
	}
	ids := map[string]workflow.ReviewItem{}
	for _, item := range review.Items {
		ids[item.ID] = item
	}
	seen := map[string]bool{}
	corrections := strings.TrimSpace(d.Reason) != "" && d.Decision == "changes_requested"
	for _, item := range d.Items {
		question, exists := ids[item.ItemID]
		if !exists || seen[item.ItemID] || utf8.RuneCountInString(item.Correction) > 4000 || !question.ValidSelection(item.SelectedOptionIDs) {
			return ErrInvalid
		}
		seen[item.ItemID] = true
		changed := len(item.SelectedOptionIDs) > 0 && !question.MatchesPlan(item.SelectedOptionIDs)
		switch item.Decision {
		case "accept":
			if strings.TrimSpace(item.Correction) != "" || changed || len(question.Options) > 0 && len(item.SelectedOptionIDs) == 0 {
				return ErrInvalid
			}
		case "revise", "reject":
			if strings.TrimSpace(item.Correction) == "" && !(item.Decision == "revise" && changed) {
				return ErrInvalid
			}
			corrections = true
		default:
			return ErrInvalid
		}
	}
	switch d.Decision {
	case "approved":
		if len(seen) != len(ids) || corrections || strings.TrimSpace(d.Reason) != "" {
			return ErrInvalid
		}
		for _, r := range review.Resolutions {
			if r.Status != "addressed" {
				return ErrInvalid
			}
		}
	case "changes_requested":
		if !corrections {
			return ErrInvalid
		}
	case "rejected", "revoked":
		if strings.TrimSpace(d.Reason) == "" {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
