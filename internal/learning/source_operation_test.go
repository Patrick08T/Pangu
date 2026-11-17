package learning

import "testing"

func TestBusinessSourceUsesRealSubmissionOperationNames(t *testing.T) {
	snapshot := []byte(`{"purpose":"business","workflow":{"project_id":"project","input":"original"}}`)
	summary := []byte(`{"project_id":"project","status":"succeeded","stop_confirmed":true}`)
	for _, operation := range []string{"submit", "fork", "rerun", "plan", "rebuild", "cron", "github"} {
		if !validBusinessRun(snapshot, summary, operation, "project") {
			t.Errorf("business operation %q rejected", operation)
		}
	}
	for _, operation := range []string{"trial", "source", "evolution-source", "evaluation", "unknown"} {
		if validBusinessRun(snapshot, summary, operation, "project") {
			t.Errorf("internal operation %q accepted", operation)
		}
	}
}
