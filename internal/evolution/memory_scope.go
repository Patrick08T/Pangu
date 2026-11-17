package evolution

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type memoryScopeIdentity struct{ role, version string }

func verifyMemoryScopeTx(ctx context.Context, tx pgx.Tx, scope Scope, prepared *catalog.PreparedCandidate, memories []workflow.Experience) error {
	if len(memories) == 0 {
		return nil
	}
	identities, err := memoryScopeIdentitiesTx(ctx, tx, scope, prepared)
	if err != nil {
		return err
	}
	for _, memory := range memories {
		var content experience.Content
		if err = json.Unmarshal(memory.Content, &content); err != nil {
			return ErrInvalid
		}
		for _, identity := range identities {
			a := content.Applicability
			if !slices.Contains(a.Roles, identity.role) || len(a.RoleVersions) > 0 && (identity.version == "" || !slices.Contains(a.RoleVersions, identity.version)) {
				return ErrForbidden
			}
		}
	}
	return nil
}

func memoryScopeIdentitiesTx(ctx context.Context, tx pgx.Tx, scope Scope, prepared *catalog.PreparedCandidate) ([]memoryScopeIdentity, error) {
	baseline := map[string]workflow.Node{}
	for _, node := range prepared.Baseline.Nodes {
		baseline[node.ID] = node
	}
	versions := map[string]*catalog.Version{}
	identities := []memoryScopeIdentity{}
	for arm, definition := range []workflow.Definition{prepared.Baseline, prepared.Candidate} {
		if len(definition.Nodes) == 0 {
			return nil, ErrForbidden
		}
		for _, node := range definition.Nodes {
			if node.AgentVersion == "" {
				return nil, ErrForbidden
			}
			version := versions[node.AgentVersion]
			if version == nil {
				var err error
				version, err = catalog.ReadVersionScopedTx(ctx, tx, scope.Namespace, node.AgentVersion, catalog.VersionScope{ProjectID: scope.ProjectID})
				if err != nil {
					return nil, err
				}
				if version.Kind != "agent" || version.ObjectID == "" {
					return nil, ErrForbidden
				}
				versions[node.AgentVersion] = version
			}
			identity := memoryScopeIdentity{role: version.ObjectID, version: version.ID}
			// Candidate compilation deliberately retains the source version ID. Changed content
			// is not that immutable version, so version-pinned memory cannot cover it implicitly.
			if arm == 1 && hashJSON(node) != hashJSON(baseline[node.ID]) {
				identity.version = ""
			}
			identities = append(identities, identity)
		}
	}
	return identities, nil
}
