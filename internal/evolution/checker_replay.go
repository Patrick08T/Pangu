package evolution

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

const replayCriterion = "research_replay_v1"

func ResearchReplayCheckerHash() string {
	return hashText("pangu/eino_research_replay_v1/checker/frozen-facts/v1;exact-statement;matching-date;required-excerpt;no-extra-facts")
}

func checkerHash(backend string) string {
	if backend == ResearchReplayBackend {
		return ResearchReplayCheckerHash()
	}
	return CheckerHash()
}

type replayEvidence struct {
	SourceID   string `json:"source_id"`
	Quote      string `json:"quote"`
	SourceDate string `json:"source_date"`
}

type replayFact struct {
	replayEvidence
	Statement string `json:"statement"`
}

func replayFacts(expected string, criteria []string) (map[string]replayFact, error) {
	if !slices.Equal(criteria, []string{replayCriterion}) {
		return nil, ErrChecker
	}
	object, err := strictObject(expected)
	if err != nil || len(object) != 1 {
		return nil, ErrChecker
	}
	facts, ok := object["facts"].([]any)
	if !ok || len(facts) < 1 || len(facts) > 30 {
		return nil, ErrChecker
	}
	required := map[string]replayFact{}
	for _, item := range facts {
		fields, ok := item.(map[string]any)
		if !ok || len(fields) != 4 {
			return nil, ErrChecker
		}
		raw, _ := json.Marshal(fields)
		var fact replayFact
		if err := json.Unmarshal(raw, &fact); err != nil {
			return nil, ErrChecker
		}
		key := normalizedFact(fact.Statement)
		_, dateErr := time.Parse("2006-01-02", fact.SourceDate)
		if key == "" || len(fact.Statement) > 2048 || strings.TrimSpace(fact.Quote) == "" || len(fact.Quote) > 1024 ||
			len(fact.SourceID) != 2 || fact.SourceID[0] != 'd' || fact.SourceID[1] < '0' || fact.SourceID[1] > '9' ||
			dateErr != nil || required[key].Statement != "" {
			return nil, ErrChecker
		}
		required[key] = fact
	}
	return required, nil
}

func normalizedFact(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func replayCaseInput(raw, expected string, criteria []string) (string, error) {
	brief, documents, err := workflow.ParseResearchReplay(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrChecker, err)
	}
	facts, err := replayFacts(expected, criteria)
	if err != nil {
		return "", err
	}
	for _, fact := range facts {
		i := int(fact.SourceID[1] - '0')
		if i >= len(documents) || !strings.Contains(documents[i], fact.Quote) {
			return "", ErrChecker
		}
	}
	return brief, nil
}

func checkReplayResult(expected string, criteria []string, actual string) (bool, bool, error) {
	required, err := replayFacts(expected, criteria)
	if err != nil {
		return false, false, err
	}
	var report struct {
		Mode   string `json:"mode"`
		Claims []struct {
			Kind      string           `json:"kind"`
			Statement string           `json:"statement"`
			Evidence  []replayEvidence `json:"evidence"`
		} `json:"claims"`
	}
	if json.Unmarshal([]byte(actual), &report) != nil || report.Mode != "research" || len(report.Claims) == 0 {
		return false, false, nil
	}
	for _, claim := range report.Claims {
		if claim.Kind != "fact" {
			continue
		}
		key := normalizedFact(claim.Statement)
		want, ok := required[key]
		if !ok || !slices.ContainsFunc(claim.Evidence, func(item replayEvidence) bool {
			return item.SourceID == want.SourceID && item.SourceDate == want.SourceDate && strings.Contains(item.Quote, want.Quote)
		}) {
			return false, true, nil
		}
		delete(required, key)
	}
	return len(required) == 0, true, nil
}
