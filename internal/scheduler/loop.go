package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func verifyWorkspaceOutput(ctx context.Context, item execution.Attempt, in workflow.NodeInput, out workflow.NodeOutput) (workflow.NodeOutput, error) {
	out.Iteration = in.Iteration
	if in.Iteration != nil {
		if len(out.Blockers) > 0 {
			return out, &workflow.Failure{Kind: "output", Cause: fmt.Errorf("循环节点不使用人工补证；质量门缺少证据应返回 passed:false")}
		}
		if !in.Iteration.Gate {
			return out, workflow.ValidateOutput(in.Node, &out)
		}
		if _, _, err := workflow.GateVerdict(out); err != nil {
			return out, &workflow.Failure{Kind: "output", Cause: err}
		}
		var err error
		out.ContentRevision, err = workspace.ContentRevision(ctx, item.RunID, in.Revision.HeadCommit)
		if err != nil {
			return out, err
		}
	}
	command, err := workspace.Verification(ctx, item.RunID)
	if err != nil {
		return out, err
	}
	if len(command) > 0 {
		evidence, verifyErr := agent.Verify(ctx, in.Workspace, command, item.ProcessScope)
		out.Evidence = append(out.Evidence, evidence)
		if verifyErr != nil {
			if ctx.Err() != nil || errors.Is(verifyErr, workflow.ErrStopUnconfirmed) || in.Iteration == nil || !in.Iteration.Gate || workflow.FailureKind(verifyErr) != "verification" || evidence.ExitCode < 1 {
				return out, verifyErr
			}
			if err = failedVerification(&out, evidence.ExitCode); err != nil {
				return out, err
			}
		}
	}
	return out, workflow.ValidateOutput(in.Node, &out)
}

func failedVerification(out *workflow.NodeOutput, code int) error {
	_, summary, err := workflow.GateVerdict(*out)
	if err != nil {
		return err
	}
	summary = fmt.Sprintf("项目验收命令退出码 %d，详见本节点工具证据。Agent 结论：%s", code, summary)
	out.Text, err = replaceVerdict(out.Text, summary)
	if err != nil {
		return err
	}
	for i := range out.Reports {
		out.Reports[i].Markdown, err = replaceVerdict(out.Reports[i].Markdown, summary)
		if err != nil {
			return err
		}
		out.ReportSHA256 = workflow.ReportHash(out.Reports[i].Markdown)
	}
	return nil
}

func replaceVerdict(body, summary string) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		return "", err
	}
	fields["passed"] = json.RawMessage(`false`)
	fields["summary"], _ = json.Marshal(summary)
	data, err := json.Marshal(fields)
	return string(data), err
}

func currentLoopRound(run *RunDetails, node NodeRun) bool {
	if node.LoopID == "" {
		return true
	}
	for _, other := range run.Nodes {
		if other.LoopID == node.LoopID && other.LoopRound > node.LoopRound {
			return false
		}
	}
	return true
}
