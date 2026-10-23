package workflow

import (
	"fmt"
	"github.com/google/uuid"
	"regexp"
	"strings"
)

type Definition struct {
	InputSchema *Shape `json:"input_schema,omitempty"`
	VersionID   string `json:"version_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	Input       string `json:"input"`
	Nodes       []Node `json:"nodes"`
}

type Node struct {
	ReadOnly          bool                    `json:"read_only,omitempty"`
	Role              string                  `json:"role,omitempty"`
	Mode              string                  `json:"mode,omitempty"`
	Policy            *Policy                 `json:"policy,omitempty"`
	OutputSchema      *Shape                  `json:"output_schema,omitempty"`
	Bindings          map[string]FieldBinding `json:"bindings,omitempty"`
	AgentVersion      string                  `json:"agent_version,omitempty"`
	ID                string                  `json:"id"`
	Executor          string                  `json:"executor"`
	Description       string                  `json:"description,omitempty"`
	Instruction       string                  `json:"instruction"`
	Model             string                  `json:"model,omitempty"`
	ConnectionVersion string                  `json:"connection_version,omitempty"`
	DependsOn         []string                `json:"depends_on,omitempty"`
}

type NodeInput struct {
	RunID        string                `json:"-"`
	NodeRunID    string                `json:"-"`
	Parameters   map[string]any        `json:"parameters,omitempty"`
	ProjectID    string                `json:"project_id,omitempty"`
	Workspace    string                `json:"-"`
	ProcessScope string                `json:"-"`
	Node         Node                  `json:"node"`
	Input        string                `json:"input"`
	Parents      map[string]NodeOutput `json:"parents"`
}

type NodeOutput struct {
	ReportIDs         []string          `json:"report_ids,omitempty"`
	Reports           []Report          `json:"-"`
	Data              map[string]any    `json:"data,omitempty"`
	Text              string            `json:"text"`
	RunID             string            `json:"run_id"`
	Evidence          []CommandEvidence `json:"-"`
	EvidenceTruncated bool              `json:"-"`
}

type CommandEvidence struct {
	ID        string `json:"id"`
	Command   string `json:"command"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func (d Definition) Layers() ([][]Node, error) {
	if d.VersionID != "" {
		if _, err := uuid.Parse(d.VersionID); err != nil {
			return nil, fmt.Errorf("version_id 必须为 UUID")
		}
	}
	if err := d.validateBindings(); err != nil {
		return nil, err
	}
	if d.ProjectID != "" {
		if _, err := uuid.Parse(d.ProjectID); err != nil {
			return nil, fmt.Errorf("project_id 必须为 UUID")
		}
	}
	if len(d.Nodes) == 0 || len(d.Nodes) > 32 {
		return nil, fmt.Errorf("节点数必须在 1 至 32 之间")
	}
	nodes := make(map[string]Node, len(d.Nodes))
	for _, n := range d.Nodes {
		if (n.Executor == "codex" || n.Role != "") && d.ProjectID == "" {
			return nil, fmt.Errorf("Codex 或专业角色编排必须指定已登记的 project_id")
		}
		if err := n.Validate(); err != nil {
			return nil, err
		}
		if _, exists := nodes[n.ID]; exists {
			return nil, fmt.Errorf("重复节点 %q", n.ID)
		}
		nodes[n.ID] = n
	}
	for _, n := range d.Nodes {
		seen := map[string]bool{}
		for _, id := range n.DependsOn {
			if _, ok := nodes[id]; !ok {
				return nil, fmt.Errorf("节点 %q 的依赖 %q 不存在", n.ID, id)
			}
			if seen[id] {
				return nil, fmt.Errorf("节点 %q 的依赖 %q 重复", n.ID, id)
			}
			seen[id] = true
		}
	}
	return d.layers()
}

func (n Node) Validate() error {
	if err := n.validateRole(); err != nil {
		return err
	}
	if err := n.validatePolicy(); err != nil {
		return err
	}
	if n.AgentVersion != "" {
		if _, err := uuid.Parse(n.AgentVersion); err != nil {
			return fmt.Errorf("agent_version 必须为 UUID")
		}
	}
	if n.OutputSchema != nil {
		if n.OutputSchema.Type != "object" {
			return fmt.Errorf("节点输出结构根类型必须为 object")
		}
		if _, err := n.OutputSchema.Compile(); err != nil {
			return err
		}
	}
	if n.ConnectionVersion != "" {
		if _, err := uuid.Parse(n.ConnectionVersion); err != nil || n.Executor != "eino" {
			return fmt.Errorf("只有 Eino 节点可引用有效的模型连接版本")
		}
	}
	if !identifier.MatchString(n.ID) {
		return fmt.Errorf("节点 ID %q 不合法", n.ID)
	}
	if n.Executor != "eino" && n.Executor != "codex" {
		return fmt.Errorf("节点 %q 的 executor 必须为 eino 或 codex", n.ID)
	}
	if (n.Executor == "eino" || n.Model != "") && (strings.TrimSpace(n.Model) == "" || len(n.Model) > 128) {
		return fmt.Errorf("节点 %q 的模型名长度必须在 1 至 128 字节之间", n.ID)
	}
	if strings.TrimSpace(n.Instruction) == "" || len(n.Instruction) > 16384 {
		return fmt.Errorf("节点 %q 的指令长度必须在 1 至 16384 字节之间", n.ID)
	}
	if len(n.Description) > 1024 {
		return fmt.Errorf("节点 %q 的描述超过 1024 字节", n.ID)
	}
	return nil
}

func (d Definition) layers() ([][]Node, error) {
	done := make(map[string]bool, len(d.Nodes))
	var layers [][]Node
	for len(done) < len(d.Nodes) {
		var layer []Node
		for _, n := range d.Nodes {
			if done[n.ID] {
				continue
			}
			ready := true
			for _, id := range n.DependsOn {
				if !done[id] {
					ready = false
				}
			}
			if ready {
				layer = append(layer, n)
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("编排包含循环依赖")
		}
		for _, n := range layer {
			done[n.ID] = true
		}
		layers = append(layers, layer)
	}
	return layers, nil
}

func (d Definition) NodeInput(n Node, outputs map[string]NodeOutput) NodeInput {
	parents := make(map[string]NodeOutput, len(n.DependsOn))
	for _, id := range n.DependsOn {
		parents[id] = outputs[id]
	}
	return NodeInput{Node: n, Input: d.Input, Parents: parents, ProjectID: d.ProjectID}
}
