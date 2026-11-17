package learning

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

// FeedbackPayload is the canonical original human feedback, not its subject hash.
type FeedbackPayload struct {
	ID            string `json:"id"`
	RunID         string `json:"run_id"`
	TaskID        string `json:"task_id"`
	ActorID       string `json:"actor_id"`
	Revision      int64  `json:"revision"`
	SubjectHash   string `json:"subject_hash"`
	Decision      string `json:"decision"`
	FirstDecision string `json:"first_decision"`
	Reason        string `json:"reason"`
}

func feedbackBody(ctx context.Context, tx pgx.Tx, scope experience.Scope, source experience.Source) ([]byte, error) {
	var out FeedbackPayload
	err := tx.QueryRow(ctx, `SELECT id,run_id,task_id,actor_id,revision,subject_hash,decision,first_decision,reason
	 FROM pangu.feedback WHERE id=$1 AND run_id=$2 AND namespace=$3 AND project_id=$4 AND NOT payload_expired FOR SHARE`,
		source.ID, source.RunID, scope.Namespace, scope.ProjectID).Scan(&out.ID, &out.RunID, &out.TaskID, &out.ActorID,
		&out.Revision, &out.SubjectHash, &out.Decision, &out.FirstDecision, &out.Reason)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func validHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}
