//go:build integration

package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
)

type rebuildPermissionBackend struct {
	privateRunBackend
	owner   string
	stopped bool
}

func (b *rebuildPermissionBackend) GetRun(_ context.Context, id string) (*scheduler.RunDetails, error) {
	return &scheduler.RunDetails{RunSummary: scheduler.RunSummary{ID: id, ProjectID: b.project, CreatedBy: b.owner, Purpose: "business"}, StopConfirmed: b.stopped}, nil
}
func (b *rebuildPermissionBackend) RebuildInvalidated(context.Context, string, string) (*submission.Record, error) {
	return nil, errors.New("must not execute rebuild in read test")
}

func TestExperienceRebuildPermissionsAreRunSpecific(t *testing.T) {
	f := experienceAPI(t)
	run := experienceLineageRun(t, f, "business")
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(context.Background(), `INSERT INTO pangu.approval_rebuild_intents(id,namespace,request_id,run_id,node_id,from_generation,to_generation,reason,requires_stop) SELECT $1,namespace,id,run_id,'node',1,2,'source withdrawn',true FROM pangu.submissions WHERE namespace=$2 AND run_id=$3`, uuid.NewString(), f.namespace, run)
	if err != nil {
		t.Fatal(err)
	}
	b := &rebuildPermissionBackend{privateRunBackend: privateRunBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, project: f.project}, owner: f.actor, stopped: true}
	h := New(b, identity.Service{Namespace: f.namespace})
	for _, test := range []struct {
		owner            string
		stopped, allowed bool
	}{{f.actor, true, true}, {uuid.NewString(), true, false}, {f.actor, false, false}} {
		b.owner, b.stopped = test.owner, test.stopped
		w := callAuthorized(h, "GET", "/runs/"+run+"/rebuild-intents", "", f.session)
		var items []struct {
			CanRebuild    bool   `json:"can_rebuild"`
			BlockedReason string `json:"blocked_reason"`
		}
		if w.Code != 200 {
			t.Fatalf("read: %d %s", w.Code, w.Body.String())
		}
		if err = json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].CanRebuild != test.allowed || !test.allowed && items[0].BlockedReason == "" {
			t.Fatalf("permission %+v: %s", test, w.Body.String())
		}
	}
	b.owner, b.stopped = f.actor, true
	f.role(t, "viewer")
	w := callAuthorized(h, "GET", "/runs/"+run+"/rebuild-intents", "", f.session)
	var items []struct {
		CanRebuild    bool   `json:"can_rebuild"`
		BlockedReason string `json:"blocked_reason"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(items) != 1 || items[0].CanRebuild || items[0].BlockedReason == "" {
		t.Fatalf("revoked execute permission: %d %s", w.Code, w.Body.String())
	}
}

func TestExperiencePurgedUnknownRequestDoesNotBecomeBusiness(t *testing.T) {
	f := experienceAPI(t)
	b := &privateRunBackend{authorizedBackend: authorizedBackend{namespace: f.namespace}, project: f.project}
	w := callAuthorized(New(b, identity.Service{Namespace: f.namespace}), "GET", "/requests/"+uuid.NewString(), "", f.session)
	if w.Code != 404 {
		t.Fatalf("purged request public: %d %s", w.Code, w.Body.String())
	}
}
