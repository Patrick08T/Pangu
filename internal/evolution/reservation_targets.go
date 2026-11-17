package evolution

import (
	"context"
	"errors"
	"slices"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/jackc/pgx/v5"
)

func targetKeys(request CandidateRequest) []string {
	keys := []string{"role:" + request.AgentObjectID}
	if request.Kind == "memory_benefit" {
		keys = append(keys, "memory:"+request.BenefitMemoryVersionID)
	}
	if request.FlowObjectID != "" {
		keys = append(keys, "flow:"+request.FlowObjectID)
	}
	for _, entry := range request.Entries {
		keys = append(keys, "entry:"+entry.Key())
	}
	slices.Sort(keys)
	return keys
}

func targetSnapshotTx(ctx context.Context, tx pgx.Tx, scope Scope, object, version, kind string) (catalog.PublishTarget, error) {
	var target catalog.PublishTarget
	var current string
	var draftOK bool
	err := tx.QueryRow(ctx, `SELECT o.id::text,o.revision,pub.id::text,CASE WHEN v.source_kind='auto' THEN v.id::text ELSE '' END,cur.id::text,(o.revision=pub.source_revision AND o.spec=pub.spec)
 FROM pangu.catalog_objects o JOIN pangu.catalog_versions v ON v.object_id=o.id
 JOIN LATERAL (SELECT x.* FROM pangu.catalog_versions x WHERE x.object_id=o.id AND x.source_kind='manual' ORDER BY number DESC LIMIT 1) pub ON true
 JOIN LATERAL (SELECT x.id FROM pangu.catalog_versions x LEFT JOIN pangu.catalog_version_state st ON st.version_id=x.id LEFT JOIN pangu.catalog_version_bodies body ON body.version_id=x.id WHERE x.object_id=o.id AND (x.source_kind='manual' OR (x.project_id=$3 AND st.state='stable' AND body.spec IS NOT NULL)) ORDER BY x.number DESC LIMIT 1) cur ON true
 WHERE o.namespace=$1 AND o.id=$2 AND v.id=$4 AND o.kind=$5 AND NOT o.archived AND o.merged_into IS NULL
 AND (v.source_kind='manual' OR (v.project_id=$3 AND v.public_baseline=pub.id)) FOR SHARE OF o`, scope.Namespace, object, scope.ProjectID, version, kind).Scan(&target.ObjectID, &target.ExpectedRevision, &target.PublicBaseline, &target.ProjectPrevious, &current, &draftOK)
	if errors.Is(err, pgx.ErrNoRows) {
		return target, ErrConflict
	}
	if err != nil {
		return target, err
	}
	if current != version || !draftOK {
		return target, ErrConflict
	}
	return target, nil
}

func (s Service) reservationEntriesTx(ctx context.Context, tx pgx.Tx, scope Scope, request CandidateRequest) ([]EntrySnapshot, error) {
	baseline := request.BaselineVersion
	if request.Payload.FlowVersion != "" {
		baseline = request.Payload.FlowVersion
	}
	entries := []EntrySnapshot{}
	for _, target := range request.Entries {
		var entry EntrySnapshot
		var err error
		if target.Kind == "workbench" {
			_, err = tx.Exec(ctx, `INSERT INTO pangu.evolution_bindings(namespace,project_id,entry_key,adopted_version,stable_version) VALUES($1,$2,$3,$4,$4) ON CONFLICT DO NOTHING`, scope.Namespace, scope.ProjectID, target.Key(), baseline)
			if err != nil {
				return nil, err
			}
			entry = EntrySnapshot{Target: target, ProjectID: scope.ProjectID, OwnerID: scope.ActorID, Enabled: true, RemoteState: "confirmed"}
			err = tx.QueryRow(ctx, `SELECT adopted_version::text,revision FROM pangu.evolution_bindings WHERE namespace=$1 AND project_id=$2 AND entry_key=$3 AND intent_id IS NULL FOR UPDATE`, scope.Namespace, scope.ProjectID, target.Key()).Scan(&entry.VersionID, &entry.Revision)
		} else if s.Entries != nil {
			entry, err = s.Entries.ReadTx(ctx, tx, scope, target)
		} else {
			return nil, ErrForbidden
		}
		if err != nil {
			return nil, err
		}
		if entry.Target != target || entry.ProjectID != scope.ProjectID || !entry.Enabled || entry.VersionID != baseline || entry.RemoteState != "confirmed" {
			return nil, ErrConflict
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func reserveClaimsTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) error {
	for _, target := range intent.Targets {
		result, err := tx.Exec(ctx, `INSERT INTO pangu.evolution_target_claims(namespace,project_id,target,intent_id,generation) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, scope.Namespace, scope.ProjectID, target, intent.ID, intent.Generation)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ErrConflict
		}
	}
	return nil
}
