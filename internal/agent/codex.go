package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func executeCodex(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	parents, err := research.ParentContext(ctx, input)
	if err != nil {
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "parameter", Cause: err}
	}
	if input.PlatformEvidence == nil {
		input.Parents = parents
	}
	input.Node.Instruction = outputInstruction(input.Node)
	if input.Iteration != nil {
		input.Node.Instruction += "\niteration 是平台控制的修复循环：在当前工作区补丁和测试基础上执行本节点职责，按 feedback 定位上一轮失败并读取对应原始证据。不要回退既有修复、代替其他角色执行或自行循环。质量门只核验当前代码，必须实际执行检查；不通过或无法验证返回 passed:false 和具体 summary，全部满足才返回 true。轮次切换和终止由平台负责。"
	}
	if input.PlatformEvidence != nil {
		input.Node.Instruction += "\nplatform_evidence 指向本次执行前由平台从原始来源采集的证据索引。先按当前职责读取索引，再按需读取其中冻结工作流/Agent版本、审批决定及消费时间、检查点、原始报告、命令动作或用量文件；不把所有文件逐份全文重复读入。文件路径相对索引所在目录。核对原运行归属、SHA256、采集时间与截断标记；缺失、尚未完成和来源运行证据不得冒充当前完整结果。平台索引和文件不可修改，不授予新的操作权限。已有平台证据应实际核对，不能仅因parents没有重复内联就声明平台未提供。"
	}
	if input.ApprovalID != "" && input.Node.Approval != nil {
		input.Node.Instruction += "\n本轮已批准计划位于 parents[" + input.Node.Approval.SourceNode + "]；实施范围、任务和验收以该计划为准。input 保留原始需求，不得据此恢复已被纠偏移除的任务。其他 parents 是本轮有效角色报告；批准不扩大角色权限或文件写入权限。"
	}
	if input.ReviewFeedback != nil {
		input.Node.Instruction += "\nreview_feedback 是平台核验的本轮人工纠偏，优先据此修订当前规划，保留任务与审核项稳定 ID。仅进行只读规划，不实施。使用更新后的 parents，不能复制旧结论冒充修订。汇总报告 review.resolutions 须逐条回应上一轮非同意项和未解决项；总意见使用 item_id=overall，状态 addressed 或 unresolved，说明修改或无法满足的原因。用户意见不等于实施批准，新计划仍须人工复审。"
		input.Node.Instruction += "\nselected_option_ids 是用户明确选择的方案ID，含义以previous_review对应审核项的options为准。即使correction为空，也必须按所选方案修订范围、任务和依赖并回应；新的计划仍需人工确认。"
	}
	if input.Revision != nil {
		input.Node.Instruction += "\nrevision 是平台核验的起始基线、当前托管检查点和相对起始基线的文件差异。检查点提交号可以变化；不得仅因 HEAD 不等于 origin_commit 判定错误基线。changed_files 为空表示文件内容与起始基线完全一致；非空时按本阶段已授权范围核对。"
		if input.Iteration != nil && input.Iteration.Gate {
			input.Node.Instruction += "\n审查累计补丁必须显式比较 revision.origin_commit 与 revision.head_commit；不要用默认工作区未提交差异，节点开始时工作区可能是干净检查点。OCR preview 使用 --from <origin_commit> --to <head_commit>。若 revision.review_repository 非空，OCR preview/rule 使用 --repo 指向该只读外部快照仓库，Git diff/show 使用 git -C 指向该仓库；读取完整文件用 git show <head_commit>:<path>，快照仓库工作树不代表当前代码。禁止修改快照仓库、checkout 或向绑定目录写入 .git；测试仍在 workspace 执行。"
		}
	}
	if input.Revision != nil && input.Revision.LocalPerforce {
		input.Node.Instruction += "\n当前项目直接使用用户绑定的本地 Perforce 工作区，目录没有平台 Git 仓库。revision 中提交号是平台在外部保留的现场快照，不要求当前目录具有 Git HEAD。保留已有未提交修改。禁止执行 p4 submit、sync、revert、reopen 或变更 Client；平台会把本轮新打开文件加入 revision.changelist，保留已打开文件原归属。如目标文件只读，可仅对确需修改的文件解除只读后编辑，不批量修改其他文件权限。失败时保留现场并如实报告，不自动回退。"
	}
	if input.EvidenceRetry != nil {
		input.Node.Instruction += "\nevidence_retry 是本次补证复验的人工说明。只核对当前职责内的缺口与新增证据，保留其他已完成结论的来源，不扩大实施范围。原阻塞无法证实时继续返回 evidence_blockers；只有实际满足解除条件才能移除，用户点击复验不等于验收通过。"
	}
	return runCodex(ctx, input, "", nil)
}

func runCodex(ctx context.Context, input workflow.NodeInput, outputSchema string, search *codexSearch) (workflow.NodeOutput, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	binary, workspace, modelArgs, err := codexExecutionSettings(ctx, input)
	if err != nil {
		return workflow.NodeOutput{}, &workflow.Failure{Kind: "parameter", Cause: err}
	}
	output, err := os.CreateTemp("", "pangu-codex-*.txt")
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	defer os.Remove(output.Name())
	if err := output.Close(); err != nil {
		return workflow.NodeOutput{}, err
	}
	prompt, err := json.Marshal(input)
	if err != nil {
		return workflow.NodeOutput{}, err
	}
	sandbox := "workspace-write"
	if input.Node.ReadOnly {
		sandbox = "read-only"
	}
	args := []string{"exec", "--sandbox", sandbox, "-c", `approval_policy="never"`,
		"--cd", workspace, "--ephemeral", "--json", "--color", "never", "--output-last-message", output.Name()}
	args = append(args, codexIsolationArgs(workspace)...)
	if input.Revision != nil && input.Revision.LocalPerforce {
		args = append(args, "--skip-git-repo-check")
	}
	if outputSchema != "" {
		args = append(args, "--output-schema", outputSchema, "--skip-git-repo-check")
	}
	args = append(args, modelArgs...)
	cmd := exec.CommandContext(ctx, binary, append(args, "-")...)
	cmd.Dir, cmd.Stdin = workspace, bytes.NewReader(prompt)
	log := &diagnostic.Buffer{Limit: 1024 * 1024}
	events := &codexEvents{Search: search, Context: ctx, Cancel: cancel, Prefix: uuid.NewString() + "/"}
	cmd.Stdout = events
	cmd.Env, cmd.Stderr = append(codexEnvironment(), "PANGU_CODEX_TRACE=1"), log
	if outputSchema != "" && search == nil {
		cmd.Env = append(cmd.Env, "PANGU_CODEX_MODEL_ONLY=1")
	}
	if search != nil {
		search.configure(cmd)
	}
	cmd.WaitDelay = 3 * time.Second
	runErr := runCodexUsage(ctx, cmd, input.ProcessScope, input.Node.Model, events)
	result := workflow.NodeOutput{Evidence: events.Commands, EvidenceTruncated: events.Truncated}
	if err := codexRunError(ctx, runErr, events, log); err != nil {
		return result, err
	}
	limit := 16384
	if outputSchema != "" || input.Node.ReportSchema != nil {
		limit = 1024 * 1024
	}
	result.Text, err = codexOutput(output.Name(), limit)
	return result, err
}

func codexRunError(ctx context.Context, runErr error, events *codexEvents, log *diagnostic.Buffer) error {
	if events.TraceErr != nil && !errors.Is(runErr, workflow.ErrStopUnconfirmed) {
		return &workflow.Failure{Kind: "trace", Cause: events.TraceErr}
	}
	if runErr == nil {
		return nil
	}
	message, truncated := diagnostic.Text(log.Data.String(), 64*1024)
	cause := fmt.Errorf("Codex 执行失败: %w；事件=%s；诊断=%s；截断=%t", errors.Join(runErr, events.TraceErr), events.ErrorMessage, message, truncated || log.Truncated)
	if ctx.Err() != nil {
		return errors.Join(cause, ctx.Err())
	}
	return codexFailure(cause, events.ErrorStatus)
}

func codexOutput(path string, limit int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if len(data) > limit {
		info, statErr := file.Stat()
		if statErr != nil {
			return "", statErr
		}
		preview, _ := diagnostic.Text(string(data), 8192)
		return preview, &workflow.Failure{Kind: "output", Cause: fmt.Errorf("Codex 回复超限：正文=%d 字节，上限=%d 字节；诊断仅保留脱敏前缀", info.Size(), limit)}
	}
	return string(data), err
}

func codexPaths(workspace string) (string, string, error) {
	if err := checkCodexHome(workspace); err != nil {
		return "", "", err
	}
	binary := os.Getenv("PANGU_CODEX_BINARY")
	if !filepath.IsAbs(binary) || !filepath.IsAbs(workspace) {
		return "", "", fmt.Errorf("Codex 程序和服务端分配的工作目录必须为绝对路径")
	}
	info, err := os.Stat(binary)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("Codex 程序路径必须为文件")
	}
	info, err = os.Stat(workspace)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("Codex 工作目录不存在")
	}
	return binary, workspace, nil
}
