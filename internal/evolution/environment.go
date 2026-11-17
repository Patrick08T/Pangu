package evolution

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s Service) RegisterEnvironment(ctx context.Context, scope Scope, input Environment, expected int64) (*Environment, error) {
	err := transaction(ctx, func(tx pgx.Tx) error {
		if err := lockScope(ctx, tx, scope); err != nil {
			return err
		}
		if err := managerTx(ctx, tx, scope); err != nil {
			return err
		}
		if input.ProjectID != scope.ProjectID || expected < 0 {
			return ErrForbidden
		}
		if _, err := uuid.Parse(input.ID); err != nil {
			return ErrInvalid
		}
		if err := configureEnvironment(ctx, tx, scope, &input); err != nil {
			return err
		}
		var revision int64
		if err := tx.QueryRow(ctx, `SELECT coalesce(max(revision),0) FROM pangu.evolution_environments WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, input.ID).Scan(&revision); err != nil {
			return err
		}
		if revision != expected {
			return ErrConflict
		}
		input.Revision = expected + 1
		input.Hash = hashJSON(input)
		_, err := tx.Exec(ctx, `INSERT INTO pangu.evolution_environments(namespace,project_id,id,revision,spec,spec_hash) VALUES($1,$2,$3,$4,$5,$6)`, scope.Namespace, scope.ProjectID, input.ID, input.Revision, jsonBytes(input), input.Hash)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &input, nil
}

func configureEnvironment(ctx context.Context, tx pgx.Tx, scope Scope, env *Environment) error {
	if env.Backend != "eino_json_v1" {
		return ErrIsolation
	}
	var data []byte
	err := tx.QueryRow(ctx, `SELECT v.spec FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id WHERE v.id=$1 AND o.namespace=$2 AND o.kind='connection' AND NOT o.archived AND v.source_kind='manual' FOR SHARE OF o`, env.ConnectionVersion, scope.Namespace).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrIsolation
	}
	if err != nil {
		return err
	}
	var spec catalog.Spec
	if err = json.Unmarshal(data, &spec); err != nil {
		return err
	}
	if spec.Connection == nil || spec.Connection.Provider == "codex" || spec.Connection.Price == nil || spec.Connection.Price.Validate() != nil || spec.Connection.Validate() != nil {
		return ErrIsolation
	}
	env.Currency = spec.Connection.Price.Currency
	env.SourceBaseline = hashText("pangu/eino_json_v1/no-tools/no-project-context/v1")
	env.ConfigHash = hashJSON(struct {
		Connection string
		Spec       *catalog.Connection
		Checker    string
	}{env.ConnectionVersion, spec.Connection, CheckerHash()})
	return nil
}

func environmentTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (*Environment, error) {
	var data []byte
	var hash string
	err := tx.QueryRow(ctx, `SELECT spec,spec_hash FROM pangu.evolution_environments WHERE namespace=$1 AND project_id=$2 AND id=$3 ORDER BY revision DESC LIMIT 1`, scope.Namespace, scope.ProjectID, id).Scan(&data, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIsolation
	}
	if err != nil {
		return nil, err
	}
	var env Environment
	if err = json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if env.ID != id || env.ProjectID != scope.ProjectID || !env.Enabled || hashJSON(env) != hash {
		return nil, ErrIsolation
	}
	original := env
	if err = configureEnvironment(ctx, tx, scope, &env); err != nil {
		return nil, err
	}
	if original != env {
		return nil, ErrIsolation
	}
	env.Hash = hash
	return &env, nil
}
