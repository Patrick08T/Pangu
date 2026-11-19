package evolution

import (
	"errors"
	"slices"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

var (
	ErrInvalid   = errors.New("进化配置或输入无效")
	ErrForbidden = errors.New("进化操作未经授权")
	ErrConflict  = errors.New("进化代际、基线或修订冲突")
	ErrNotFound  = errors.New("进化记录不存在")
	ErrIsolation = errors.New("没有获准的受控执行环境")
	ErrChecker   = errors.New("缺少受信确定性检查器")
	ErrBudget    = errors.New("独立学习或评测预算不足")
)

type Environment struct {
	ID                string `json:"id"`
	ProjectID         string `json:"project_id"`
	Revision          int64  `json:"revision"`
	Backend           string `json:"backend"`
	ConnectionVersion string `json:"connection_version"`
	Enabled           bool   `json:"enabled"`
	Currency          string `json:"currency"`
	SourceBaseline    string `json:"source_baseline"`
	ConfigHash        string `json:"config_hash"`
	Hash              string `json:"-"`
}

const ResearchReplayBackend = "eino_research_replay_v1"

func ValidateExecution(env Environment, d workflow.Definition) error {
	replay := env.Backend == ResearchReplayBackend
	if !env.Enabled || (!replay && env.Backend != "eino_json_v1") || env.ConnectionVersion == "" || len(d.Nodes) == 0 {
		return ErrIsolation
	}
	for _, n := range d.Nodes {
		if n.Executor != "eino" || n.ConnectionVersion != env.ConnectionVersion || n.Approval != nil || (!replay && (n.Policy != nil || n.Role != "")) {
			return ErrIsolation
		}
	}
	if _, err := d.Layers(); err != nil {
		return ErrIsolation
	}
	if replay {
		_, err := ReplayCollector(d)
		return err
	}
	return nil
}

func ReplayCollector(d workflow.Definition) (workflow.Node, error) {
	var selected workflow.Node
	if len(d.Loops) > 0 {
		return selected, ErrIsolation
	}
	for _, n := range d.Nodes {
		if n.Role == "" {
			continue
		}
		if selected.ID != "" || n.Role != "product_manager" || n.Mode != "research" || n.When != nil || n.ReportSchema != nil || len(n.DependsOn)+len(n.Bindings) > 0 {
			return selected, ErrIsolation
		}
		selected = n
	}
	if selected.ID == "" {
		return selected, ErrIsolation
	}
	return selected, nil
}

func validateTargets(env Environment, d workflow.Definition, request CandidateRequest) error {
	if err := ValidateExecution(env, d); err != nil {
		return err
	}
	if env.Backend != ResearchReplayBackend {
		return nil
	}
	node, err := ReplayCollector(d)
	if err != nil || node.AgentVersion != request.BaselineVersion ||
		(request.Payload.FlowVersion != "" && !slices.Equal(request.Payload.BindingIDs, []string{node.ID})) {
		return ErrIsolation
	}
	return nil
}
