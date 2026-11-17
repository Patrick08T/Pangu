package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

type CandidatePayload struct {
	Agent       workflow.Node `json:"agent"`
	FlowVersion string        `json:"flow_version,omitempty"`
	BindingIDs  []string      `json:"binding_ids,omitempty"`
}

type PreparedCandidate struct {
	Baseline              workflow.Definition
	Candidate             workflow.Definition
	BaselineHash          string
	CandidateHash         string
	BaselineCompiledHash  string
	CandidateCompiledHash string
}

type versionLookup func(string) (*Version, error)

// UpgradeFlowReferences 深复制完整 Flow，只替换明确授权且仍指向基线的绑定。
func UpgradeFlowReferences(flow *Flow, baseline, replacement string, bindings []string) (*Flow, error) {
	if flow == nil || baseline == "" || replacement == "" || len(bindings) == 0 {
		return nil, invalid("缺少工作流升级范围")
	}
	data, err := json.Marshal(flow)
	if err != nil {
		return nil, err
	}
	var out Flow
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, id := range bindings {
		if id == "" || allowed[id] {
			return nil, invalid("工作流升级绑定重复或为空")
		}
		allowed[id] = true
	}
	for i := range out.Nodes {
		node := &out.Nodes[i]
		if !allowed[node.ID] {
			continue
		}
		if node.AgentVersion != baseline {
			return nil, ErrConflict
		}
		node.AgentVersion = replacement
		delete(allowed, node.ID)
	}
	if len(allowed) != 0 {
		return nil, invalid("工作流升级绑定不存在")
	}
	return &out, nil
}

func prepareCandidate(ctx context.Context, project, baseline string, payload CandidatePayload, lookup versionLookup) (*PreparedCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(project); err != nil {
		return nil, invalid("实验必须指定有效项目")
	}
	base, err := lookup(baseline)
	if err != nil {
		return nil, err
	}
	if base.Kind != "agent" || base.Spec.Agent == nil || base.Archived {
		return nil, invalid("实验基线角色不可用")
	}
	if err = ValidateInstructionCandidate(*base.Spec.Agent, payload.Agent); err != nil {
		return nil, err
	}
	before, err := compileCandidateArm(project, base, payload, false, lookup)
	if err != nil {
		return nil, err
	}
	after, err := compileCandidateArm(project, base, payload, true, lookup)
	if err != nil {
		return nil, err
	}
	return &PreparedCandidate{Baseline: *before, Candidate: *after,
		BaselineHash: contentHash(base.Spec.Agent), CandidateHash: contentHash(payload.Agent),
		BaselineCompiledHash: CompiledHash(*before), CandidateCompiledHash: CompiledHash(*after)}, nil
}

func compileCandidateArm(project string, base *Version, payload CandidatePayload, candidate bool, lookup versionLookup) (*workflow.Definition, error) {
	node := *base.Spec.Agent
	if candidate {
		node = payload.Agent
	}
	node.AgentVersion = base.ID
	out := &workflow.Definition{ProjectID: project, VersionID: base.ID, Nodes: []workflow.Node{node}}
	if payload.FlowVersion != "" {
		flow, err := lookup(payload.FlowVersion)
		if err != nil {
			return nil, err
		}
		if flow.Kind != "workflow" || flow.Spec.Flow == nil || flow.Archived {
			return nil, invalid("实验工作流不可用")
		}
		key := "candidate:" + base.ID
		copied, err := UpgradeFlowReferences(flow.Spec.Flow, base.ID, key, payload.BindingIDs)
		if err != nil {
			return nil, err
		}
		out, err = resolveFlowLookup(project, copied, true, func(id string) (*Version, error) {
			if id == key {
				return &Version{ID: base.ID, Kind: "agent", Spec: Spec{Agent: &node}}, nil
			}
			return lookup(id)
		})
		if err != nil {
			return nil, err
		}
		out.VersionID = flow.ID
	} else if len(payload.BindingIDs) != 0 {
		return nil, invalid("独立角色实验不能指定流程绑定")
	}
	if err := out.ValidateHandoffs(); err != nil {
		return nil, invalid(err.Error())
	}
	if _, err := out.Layers(); err != nil {
		return nil, invalid(err.Error())
	}
	if err := validateFields(out); err != nil {
		return nil, err
	}
	return out, nil
}

// CompiledHash 固定实际编译内容，样例输入另由评测执行身份绑定。
func CompiledHash(d workflow.Definition) string {
	d.Input = ""
	return contentHash(d)
}

func contentHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
