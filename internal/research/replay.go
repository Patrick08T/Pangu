package research

import (
	"errors"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func NewReplay(input workflow.NodeInput) (*Session, string, error) {
	if input.Node.Executor != "eino" || input.Node.Role != "product_manager" || input.Node.Mode != "research" || input.Workspace != "" || len(input.Parents)+len(input.Parameters) > 0 {
		return nil, "", errors.New("回放只能执行无工作区及上游输入的 Eino 调研节点")
	}
	brief, documents, err := workflow.ParseResearchReplay(input.Input)
	if err != nil {
		return nil, "", err
	}
	for _, doc := range documents {
		if diagnostic.Sensitive([]byte(doc)) {
			return nil, "", errors.New("冻结资料疑似包含凭据")
		}
	}
	input.Input = brief
	session, err := New(input, nil)
	if err != nil {
		return nil, "", err
	}
	session.replayDocs = documents
	return session, brief, nil
}
