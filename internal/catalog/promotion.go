package catalog

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

type PublishTarget struct {
	ObjectID         string
	ExpectedRevision int
	PublicBaseline   string
	ProjectPrevious  string
}

type AutomaticPublishRequest struct {
	IntentID     string
	ExperimentID string
	Agent        PublishTarget
	Flow         *PublishTarget
}

type AutomaticPublication struct {
	Agent Version
	Flow  *Version
}

// PublishGuard 必须在传入的事务核对实时授权、策略、晋级intent及目标占用。
type PublishGuard func(context.Context, pgx.Tx, ExperimentIdentity, AutomaticPublishRequest) error

func PublishAutomatic(ctx context.Context, namespace string, request AutomaticPublishRequest, guard PublishGuard) (*AutomaticPublication, error) {
	if guard == nil {
		return nil, invalid("自动发布缺少事务授权守卫")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	out, err := PublishAutomaticTx(ctx, tx, namespace, request, guard)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// PublishAutomaticTx 将晋级意图和版本原子提交。guard 必须先锁身份/策略/占用，不能依赖请求中的布尔授权。
func PublishAutomaticTx(ctx context.Context, tx pgx.Tx, namespace string, request AutomaticPublishRequest, guard PublishGuard) (*AutomaticPublication, error) {
	if guard == nil || request.IntentID == "" || request.ExperimentID == "" {
		return nil, invalid("自动发布缺少事务授权守卫或意图")
	}
	e, err := guardPromotion(ctx, tx, namespace, request, guard)
	if err != nil {
		return nil, err
	}
	existing, err := existingPublication(ctx, tx, namespace, *e, request)
	if err != nil || existing != nil {
		return existing, err
	}
	objects, err := lockPublishObjects(ctx, tx, namespace, request)
	if err != nil {
		return nil, err
	}
	_, _, payload, err := loadExperimentCandidate(ctx, tx, namespace, e.ID)
	if err != nil {
		return nil, err
	}
	if (payload.FlowVersion == "") != (request.Flow == nil) {
		return nil, ErrConflict
	}
	agent, err := publishTarget(ctx, tx, namespace, *e, objects[request.Agent.ObjectID], request.Agent, e.BaselineVersion, Spec{Agent: &payload.Agent})
	if err != nil {
		return nil, err
	}
	out := &AutomaticPublication{Agent: *agent}
	if request.Flow != nil {
		flow, err := readVersionScoped(ctx, tx, namespace, payload.FlowVersion, VersionScope{ProjectID: e.LogicalProjectID})
		if err != nil {
			return nil, err
		}
		upgraded, err := UpgradeFlowReferences(flow.Spec.Flow, e.BaselineVersion, agent.ID, payload.BindingIDs)
		if err != nil {
			return nil, err
		}
		out.Flow, err = publishTarget(ctx, tx, namespace, *e, objects[request.Flow.ObjectID], *request.Flow, payload.FlowVersion, Spec{Flow: upgraded})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func validatePublishTarget(item Object, base, public, current Version, target PublishTarget) error {
	if item.Archived {
		return ErrArchived
	}
	if item.ID != target.ObjectID || base.ObjectID != item.ID || item.Revision != target.ExpectedRevision || public.ID != target.PublicBaseline || current.ID != base.ID || public.SourceRevision != item.Revision || !sameJSON(item.Spec, public.Spec) {
		return ErrConflict
	}
	previous := ""
	if base.SourceKind == "auto" {
		previous = base.ID
		if base.PublicBaseline != public.ID {
			return ErrConflict
		}
	} else if base.ID != public.ID {
		return ErrConflict
	}
	if previous != target.ProjectPrevious {
		return ErrConflict
	}
	return nil
}
