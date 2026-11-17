package experience

import "encoding/json"

func selectionIdentity(input SelectionInput) string {
	identity := struct {
		ProjectID, RunID, NodeID, BoundInputHash string
		Round, Generation, ApprovalRevision      int64
	}{input.ProjectID, input.RunID, input.NodeID, input.BoundInputHash,
		int64(input.Round), input.Generation, input.ApprovalRevision}
	data, _ := json.Marshal(identity)
	return hashBytes(data)
}
