//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func environmentConnection(t *testing.T, f governanceFixture) *catalog.Version {
	t.Helper()
	ctx := context.Background()
	item, err := catalog.Create(ctx, f.scope.Namespace, catalog.NewObject{Kind: "connection", Name: "eino test", Spec: catalog.Spec{Connection: &catalog.Connection{Provider: "openai", Price: &workflow.Price{Currency: "USD", InputPerMillionMicros: 1000000, OutputPerMillionMicros: 2000000}, BaseURL: "https://example.invalid/v1", Model: "offline-fixture", SecretRef: "PANGU_SECRET_EVOLUTION_TEST"}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, f.scope.Namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func TestEnvironmentRegistryFreezesActualCapabilityAndChecksRevocation(t *testing.T) {
	f := newGovernanceFixture(t)
	connection := environmentConnection(t, f)
	service := Service{}
	input := Environment{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", ConnectionVersion: connection.ID, Enabled: true, SourceBaseline: "caller-forged", ConfigHash: "caller-forged"}
	env, err := service.RegisterEnvironment(context.Background(), f.scope, input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if env.Revision != 1 || env.Hash == "" || env.SourceBaseline == input.SourceBaseline || env.ConfigHash == input.ConfigHash {
		t.Fatal("调用方替代真实能力注册", env)
	}
	if _, err = service.RegisterEnvironment(context.Background(), f.scope, input, 0); !errors.Is(err, ErrConflict) {
		t.Fatal("过期环境CAS被接受", err)
	}
	input.Enabled = false
	disabled, err := service.RegisterEnvironment(context.Background(), f.scope, input, 1)
	if err != nil || disabled.Revision != 2 {
		t.Fatal(disabled, err)
	}
	err = governanceTx(f, func(ctx context.Context, tx pgx.Tx) error {
		_, err := environmentTx(ctx, tx, f.scope, env.ID)
		return err
	})
	if !errors.Is(err, ErrIsolation) {
		t.Fatal("禁用环境仍可执行", err)
	}
}

func TestEnvironmentRegistryRejectsUnpricedConnection(t *testing.T) {
	f := newGovernanceFixture(t)
	version := environmentConnection(t, f)
	version.Spec.Connection.Price = nil
	item, err := catalog.Create(context.Background(), f.scope.Namespace, catalog.NewObject{Kind: "connection", Name: "unpriced", Spec: version.Spec})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := catalog.Publish(context.Background(), f.scope.Namespace, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (Service{}).RegisterEnvironment(context.Background(), f.scope, Environment{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Backend: "eino_json_v1", ConnectionVersion: unknown.ID, Enabled: true}, 0)
	if !errors.Is(err, ErrIsolation) {
		t.Fatal("未知费用仍可启用评测", err)
	}
}

func TestEnvironmentRegistryRejectsCodexAndMissingManager(t *testing.T) {
	f := newGovernanceFixture(t)
	service := Service{}
	input := Environment{ID: uuid.NewString(), ProjectID: f.scope.ProjectID, Backend: "codex", ConnectionVersion: uuid.NewString(), Enabled: true}
	if _, err := service.RegisterEnvironment(context.Background(), f.scope, input, 0); !errors.Is(err, ErrIsolation) {
		t.Fatal("Codex未隔离却注册成功", err)
	}
	input.Backend = "eino_json_v1"
	f.scope.ActorID = uuid.NewString()
	if _, err := service.RegisterEnvironment(context.Background(), f.scope, input, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("非管理者注册环境", err)
	}
}
