//go:build integration

package scheduler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

func maintenanceBackend(t *testing.T) Backend {
	t.Helper()
	return maintenanceBackendHTTP(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
}

func maintenanceBackendHTTP(t *testing.T, handler http.Handler) Backend {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	claims, _ := json.Marshal(map[string]any{"server_url": server.URL, "grpc_broadcast_address": "127.0.0.1:1", "exp": time.Now().Add(time.Hour).Unix(), "sub": uuid.NewString()})
	token := "test." + base64.RawURLEncoding.EncodeToString(claims) + ".fixture"
	t.Setenv("HATCHET_CLIENT_SERVER_URL", server.URL)
	t.Setenv("HATCHET_CLIENT_TOKEN", token)
	t.Setenv("HATCHET_CLIENT_TENANT_ID", uuid.NewString())
	client, err := hatchet.NewClient(hatchet.WithToken(token), hatchet.WithNamespace("maintenance-"+uuid.NewString()), hatchet.WithHostPort("127.0.0.1", 1), hatchet.WithTLSConfig(nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	return NewBackend(client)
}

func TestExperienceMaintenanceLoopPausesDisabledPolicyDelivery(t *testing.T) {
	entryDB(t)
	b := maintenanceBackend(t)
	ns, project, actor := b.Namespace(), uuid.NewString(), uuid.NewString()
	maintenanceProject(t, ns, project)
	entryExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, actor, ns)
	id := maintenanceMemory(t, ns, project, false, true, true)
	entryExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,1,$3,'{"enabled":false,"read_enabled":false}')`, ns, project, actor)
	entryExec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,1,'canary','{"mode":"canary"}',$2)`, id, actor)
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	b.Maintain(ctx)
	var mode string
	if err := entryQuery(t, `SELECT mode FROM pangu.experience_deliveries WHERE version_id=$1 ORDER BY revision DESC LIMIT 1`, id).Scan(&mode); err != nil || mode != "paused" {
		t.Fatalf("maintenance did not pause disabled delivery: %s %v", mode, err)
	}
	if body, tombstone := maintenanceMemoryState(t, id); !body || tombstone {
		t.Fatal("pause was confused with body retention")
	}
}

func TestExperienceMaintenanceRetentionPreviewPreservesBodies(t *testing.T) {
	entryDB(t)
	b := maintenanceBackend(t)
	ns, project := b.Namespace(), uuid.NewString()
	maintenanceProject(t, ns, project)
	id := maintenanceMemory(t, ns, project, true, true, true)
	if _, err := b.Retention(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if body, tombstone := maintenanceMemoryState(t, id); !body || tombstone {
		t.Fatal("retention preview erased memory")
	}
	if _, err := b.Retention(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if body, tombstone := maintenanceMemoryState(t, id); body || !tombstone {
		t.Fatal("retention command did not invoke memory body maintenance")
	}
}
