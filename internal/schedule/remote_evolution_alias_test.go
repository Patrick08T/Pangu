package schedule

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Evolution's EntryTarget retains the caller's UUID spelling while target_id is UUID.
func TestRemoteEvolutionOwnerTargetUUIDAliasesRemainOwned(t *testing.T) {
	for _, form := range []string{"uppercase", "compact", "braces", "urn"} {
		t.Run(form, func(t *testing.T) {
			ctx, namespace, plan, conn := remoteEvolutionFixture(t)
			key := remoteEvolutionIntent(t, conn, namespace, plan, "pending")
			alias := strings.ToUpper(plan.ID)
			switch form {
			case "compact":
				alias = strings.ReplaceAll(alias, "-", "")
			case "braces":
				alias = "{" + alias + "}"
			case "urn":
				alias = "urn:uuid:" + alias
			}
			remoteEvolutionExec(t, conn, `UPDATE pangu.evolution_remote_intents SET result=jsonb_set(result,'{target,id}',to_jsonb($2::text)) WHERE effect_key=$1`, key, alias)
			ids, err := RemotePending(ctx, namespace)
			if err != nil || slices.Contains(ids, plan.ID) {
				t.Errorf("owner UUID alias bypassed pending exclusion: %v %v", ids, err)
			}
			result, err := ReconcileRemote(ctx, namespace, plan.ID, func(context.Context, *Plan) (string, error) {
				t.Error("owner UUID alias bypassed actual exclusion")
				return uuid.NewString(), nil
			})
			if !errors.Is(err, ErrConflict) || result == nil || result.Ready {
				t.Fatalf("owner alias confirmed: %+v %v", result, err)
			}
		})
	}
}
