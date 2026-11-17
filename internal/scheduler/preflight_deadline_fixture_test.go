package scheduler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

// The transport test never connects to a real Hatchet server or product database.
func deadlineBackend(t *testing.T) Backend {
	t.Helper()
	server := "http://127.0.0.1:1"
	claims, err := json.Marshal(map[string]any{"server_url": server, "grpc_broadcast_address": "127.0.0.1:1", "exp": time.Now().Add(time.Hour).Unix(), "sub": uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	token := "test." + base64.RawURLEncoding.EncodeToString(claims) + ".fixture"
	t.Setenv("HATCHET_CLIENT_SERVER_URL", server)
	t.Setenv("HATCHET_CLIENT_TOKEN", token)
	t.Setenv("HATCHET_CLIENT_TENANT_ID", uuid.NewString())
	client, err := hatchet.NewClient(hatchet.WithToken(token), hatchet.WithNamespace("deadline-"+uuid.NewString()), hatchet.WithHostPort("127.0.0.1", 1), hatchet.WithTLSConfig(nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	return NewBackend(client)
}
