//go:build integration

package scheduler

import (
	"testing"

	"github.com/google/uuid"
)

func TestEvolutionConsumersPreservesCurrentHumanBindings(t *testing.T) {
	for _, kind := range []string{"workbench", "stable", "paused_cron", "disabled_hook"} {
		t.Run(kind, func(t *testing.T) {
			f := newEvolutionConsumersFixture(t)
			id, version := uuid.NewString(), f.target.VersionIDs[0]
			switch kind {
			case "workbench", "stable":
				adopted, stable := version, f.request.BaselineVersion
				if kind == "stable" {
					adopted, stable = stable, adopted
				}
				f.exec(t, `INSERT INTO pangu.evolution_bindings(namespace,project_id,entry_key,adopted_version,stable_version) VALUES($1,$2,$3,$4,$5)`, f.scope.Namespace, f.scope.ProjectID, "workbench:manual", adopted, stable)
			case "paused_cron":
				f.exec(t, `INSERT INTO pangu.schedules(id,namespace,project_id,name,expression,workflow,paused) VALUES($1,$2,$3,'manual','0 * * * *',$4,true)`, id, f.scope.Namespace, f.scope.ProjectID, evolutionJSON(map[string]any{"nodes": []any{map[string]any{"agent_version": version}}}))
			case "disabled_hook":
				f.exec(t, `INSERT INTO pangu.github_hooks(id,namespace,project_id,actor_id,version_id,repository,label,secret_ref,enabled) VALUES($1,$2,$3,$4,$5,'owner/repo','manual','test-secret-ref',false)`, id, f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, version)
			}
			if ok, err := f.guard(t, "unreferenced"); ok || err != nil {
				t.Fatal("human binding was ignored", kind, ok, err)
			}
		})
	}
}

func TestEvolutionConsumersFindsFlowRoleDependency(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	object, version := uuid.NewString(), uuid.NewString()
	spec := evolutionJSON(map[string]any{"flow": map[string]any{"nodes": []any{map[string]any{"agent_version": f.target.VersionIDs[0]}}}})
	f.exec(t, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1,$2,'workflow','manual dependency',$3)`, object, f.scope.Namespace, spec)
	f.exec(t, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) VALUES($1,$2,1,1,$3)`, version, object, spec)
	item := f.submit(t, version, false, false)
	if ok, err := f.guard(t, "stopped"); ok || err == nil {
		t.Fatal("flow role dependency omitted", ok, err)
	}
	f.stop(t, item.RunID)
	if ok, err := f.guard(t, "stopped"); !ok || err != nil {
		t.Fatal("flow consumer stop rejected", ok, err)
	}
	if ok, err := f.guard(t, "unreferenced"); ok || err != nil {
		t.Fatal("manual flow dependency ignored", ok, err)
	}
}

func TestEvolutionConsumersUnreferencedWithoutFutureConsumers(t *testing.T) {
	f := newEvolutionConsumersFixture(t)
	if ok, err := f.guard(t, "unreferenced"); !ok || err != nil {
		t.Fatal("unbound root body cannot clear", ok, err)
	}
}
