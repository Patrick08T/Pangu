package catalog

import (
	"context"
	"regexp"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Bundle struct {
	Name       string           `json:"name"`
	ObjectID   string           `json:"object_id,omitempty"`
	Revision   int              `json:"revision,omitempty"`
	Agents     []AgentChange    `json:"agents"`
	Steps      []BundleStep     `json:"steps,omitempty"`
	Parameters *workflow.Shape  `json:"parameters,omitempty"`
	Budget     *workflow.Budget `json:"budget,omitempty"`
}

type AgentChange struct {
	Key       string         `json:"key"`
	Name      string         `json:"name"`
	ObjectID  string         `json:"object_id,omitempty"`
	Revision  int            `json:"revision,omitempty"`
	VersionID string         `json:"version_id,omitempty"`
	Node      *workflow.Node `json:"node,omitempty"`
}

type BundleStep struct {
	ID        string                           `json:"id"`
	Agent     string                           `json:"agent"`
	DependsOn []string                         `json:"depends_on,omitempty"`
	Bindings  map[string]workflow.FieldBinding `json:"bindings,omitempty"`
	When      *workflow.Condition              `json:"when,omitempty"`
	Approval  *workflow.Approval               `json:"approval,omitempty"`
}

type BundleResult struct {
	Agents   []Version `json:"agents"`
	Workflow *Version  `json:"workflow,omitempty"`
}

var bundleKey = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func (b Bundle) Validate() error {
	if len(b.Agents) < 1 || len(b.Agents) > 12 || len(b.Steps) > 32 {
		return invalid("候选需要 1—12 个 Agent，最多 32 个步骤")
	}
	keys, targets := map[string]bool{}, map[string]bool{}
	for _, a := range b.Agents {
		if !bundleKey.MatchString(a.Key) || keys[a.Key] {
			return invalid("Agent 标识无效或重复")
		}
		keys[a.Key] = true
		if a.VersionID != "" {
			if _, err := uuid.Parse(a.VersionID); err != nil || a.Node != nil || a.ObjectID != "" || a.Revision != 0 {
				return invalid("复用版本不能同时修改 Agent")
			}
		} else if a.Node == nil {
			return invalid("新建或修改 Agent 必须提供配置")
		}
		if err := bundleTarget(a.ObjectID, a.Revision); err != nil {
			return err
		}
		if a.ObjectID != "" && targets[a.ObjectID] {
			return invalid("不能重复修改同一 Agent")
		}
		targets[a.ObjectID] = true
	}
	for _, s := range b.Steps {
		if !keys[s.Agent] {
			return invalid("步骤引用未知 Agent")
		}
	}
	if len(b.Steps) == 0 && (b.ObjectID != "" || b.Parameters != nil || b.Budget != nil) {
		return invalid("单 Agent 候选不能含工作流配置")
	}
	return bundleTarget(b.ObjectID, b.Revision)
}

func bundleTarget(id string, revision int) error {
	if id == "" && revision == 0 {
		return nil
	}
	if _, err := uuid.Parse(id); err != nil || revision < 1 {
		return invalid("修改对象必须指定有效 ID 和修订号")
	}
	return nil
}

// CheckBundle 复用真正的发布路径，但始终回滚，不留下预览对象。
func CheckBundle(ctx context.Context, ns string, b Bundle) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	_, err = ApplyBundleTx(ctx, tx, ns, b)
	return err
}

func ApplyBundleTx(ctx context.Context, tx pgx.Tx, ns string, b Bundle) (*BundleResult, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	// 工作空间事务锁固定批量变更次序，避免两个候选交叉锁定对象。
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "catalog-bundle:"+ns); err != nil {
		return nil, err
	}
	out, versions := &BundleResult{Agents: []Version{}}, map[string]string{}
	for _, a := range b.Agents {
		v, err := bundleAgent(ctx, tx, ns, a)
		if err != nil {
			return nil, err
		}
		out.Agents = append(out.Agents, *v)
		versions[a.Key] = v.ID
	}
	if len(b.Steps) > 0 {
		flow := &Flow{Parameters: b.Parameters, Budget: b.Budget}
		for _, s := range b.Steps {
			flow.Nodes = append(flow.Nodes, Binding{ID: s.ID, AgentVersion: versions[s.Agent], DependsOn: s.DependsOn, Bindings: s.Bindings, When: s.When, Approval: s.Approval})
		}
		v, err := bundlePublish(ctx, tx, ns, NewObject{Kind: "workflow", Name: b.Name, Spec: Spec{Flow: flow}}, b.ObjectID, b.Revision)
		if err != nil {
			return nil, err
		}
		out.Workflow = v
	}
	return out, nil
}

func bundleAgent(ctx context.Context, tx pgx.Tx, ns string, a AgentChange) (*Version, error) {
	if a.VersionID == "" {
		return bundlePublish(ctx, tx, ns, NewObject{Kind: "agent", Name: a.Name, Spec: Spec{Agent: a.Node}}, a.ObjectID, a.Revision)
	}
	v, err := readVersion(ctx, tx, ns, a.VersionID)
	if err != nil {
		return nil, err
	}
	if v.Kind != "agent" || v.Archived {
		return nil, invalid("只能复用工作空间未归档的 Agent 版本")
	}
	if err := v.Spec.Agent.Handoff.Validate(true); err != nil {
		return nil, invalid(err.Error())
	}
	return v, nil
}

func bundlePublish(ctx context.Context, tx pgx.Tx, ns string, input NewObject, id string, revision int) (*Version, error) {
	var item *Object
	var err error
	if id == "" {
		item, err = createObject(ctx, tx, ns, input)
	} else {
		previous, e := lockedObject(ctx, tx, ns, id, revision)
		if e != nil {
			return nil, e
		}
		if previous.Kind != input.Kind {
			return nil, invalid("不能改变对象种类")
		}
		item, err = saveObject(ctx, tx, ns, id, revision, input.Name, input.Spec)
	}
	if err != nil {
		return nil, err
	}
	return publishObject(ctx, tx, ns, item.ID, item.Revision)
}
