//go:build integration

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
)

func TestExperienceCapabilitiesMatchCurrentGovernanceAuthority(t *testing.T) {
	for _, test := range []struct {
		name, role            string
		admin, manage, submit bool
		writeStatus           int
	}{
		{"manager", "manager", false, true, true, 200},
		{"operator", "operator", false, false, true, 403},
		{"viewer", "viewer", false, false, false, 403},
		{"admin_without_membership", "viewer", true, true, true, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := experienceAPI(t)
			f.role(t, test.role)
			if test.admin {
				capabilityExec(t, `UPDATE pangu.users SET is_admin=true WHERE id=$1`, f.actor)
				capabilityExec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1 AND project_id=$2`, f.actor, f.project)
			}
			h := capabilityHandler(f, true)
			got := capabilityRead(t, h, f)
			capabilityPermissions(t, got, test.manage, test.submit, test.manage)
			if !got.Features["entry_submission"] || got.Features["evolution"] {
				t.Fatal("runtime readiness was coupled to caller role")
			}
			w := callAuthorized(h, "PUT", "/projects/"+f.project+"/experience-policy", `{"expected_revision":0,"policy":{"enabled":false,"read_enabled":false}}`, f.session)
			if w.Code != test.writeStatus {
				t.Fatalf("real policy write status=%d want=%d body=%s", w.Code, test.writeStatus, w.Body.String())
			}
		})
	}
}

func TestExperienceCapabilitiesDoNotCacheRolesOrConfuseReadiness(t *testing.T) {
	f := experienceAPI(t)
	for _, test := range []struct {
		role           string
		manage, submit bool
	}{{"manager", true, true}, {"operator", false, true}, {"viewer", false, false}, {"manager", true, true}} {
		f.role(t, test.role)
		for _, ready := range []bool{false, true} {
			got := capabilityRead(t, capabilityHandler(f, ready), f)
			capabilityPermissions(t, got, test.manage, test.submit, test.manage && ready)
			if got.Features["entry_submission"] != ready || got.Features["evolution"] || !got.Features["memory_read"] {
				t.Fatalf("readiness changed with %s permission: %+v", test.role, got.Features)
			}
		}
	}
}

func TestExperienceCapabilitiesRejectLostOrForeignVisibility(t *testing.T) {
	for _, mutation := range []string{"disabled_user", "disabled_membership", "foreign_project", "foreign_namespace"} {
		t.Run(mutation, func(t *testing.T) {
			f := experienceAPI(t)
			want := 403
			switch mutation {
			case "disabled_user":
				capabilityExec(t, `UPDATE pangu.users SET disabled=true WHERE id=$1`, f.actor)
				want = 401
			case "disabled_membership":
				capabilityExec(t, `UPDATE pangu.memberships SET enabled=false WHERE user_id=$1 AND project_id=$2`, f.actor, f.project)
			case "foreign_project":
				f.project = f.other
			case "foreign_namespace":
				capabilityExec(t, `UPDATE pangu.projects SET namespace=$2 WHERE id=$1`, f.project, uuid.NewString())
			}
			w := callAuthorized(f.handler, "GET", "/projects/"+f.project+"/experience-capabilities", "", f.session)
			if w.Code != want {
				t.Fatalf("visibility status=%d want=%d body=%s", w.Code, want, w.Body.String())
			}
		})
	}
}

func TestExperienceCapabilitiesTokensCannotClaimHumanGovernanceOrEntry(t *testing.T) {
	f := experienceAPI(t)
	for _, scopes := range [][]string{{"read"}, {"read", "run"}} {
		_, token, err := identity.IssueToken(context.Background(), identity.Actor{ID: f.actor, Namespace: f.namespace}, f.project, "capability test", scopes, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		h := capabilityHandler(f, true)
		w := capabilityBearer(h, "/projects/"+f.project+"/experience-capabilities", token)
		var got capabilityResponse
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil {
			t.Fatalf("token read=%d %s", w.Code, w.Body.String())
		}
		capabilityPermissions(t, got, false, false, false)
		if !got.Features["entry_submission"] || got.Features["evolution"] {
			t.Fatal("token permission hid runtime readiness")
		}
		w = capabilityBearer(h, "/projects/"+f.other+"/experience-capabilities", token)
		if w.Code != 403 {
			t.Fatalf("cross-project token status=%d", w.Code)
		}
	}
}

type capabilityCancellationAccess struct {
	identity.Service
	cancel context.CancelFunc
}

func (a capabilityCancellationAccess) Authorize(ctx context.Context, project, action, owner string) error {
	err := a.Service.Authorize(ctx, project, action, owner)
	if action == "read" && err == nil {
		a.cancel()
	}
	return err
}

func TestExperienceCapabilitiesDoNotHideInfrastructureErrorsAsDenial(t *testing.T) {
	f := experienceAPI(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := New(&authorizedBackend{namespace: f.namespace}, capabilityCancellationAccess{Service: identity.Service{Namespace: f.namespace}, cancel: cancel})
	request := httptest.NewRequest("GET", "/projects/"+f.project+"/experience-capabilities", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: "pangu_session", Value: f.session})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("canceled authority became permission response: %d %s", response.Code, response.Body.String())
	}
}

func TestExperienceCapabilitiesAuthorityPropagatesDatabaseFailure(t *testing.T) {
	f := experienceAPI(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowed, err := experienceGovernanceAllowed(ctx, experience.Scope{Namespace: f.namespace, ProjectID: f.project, ActorID: f.actor})
	if allowed || err == nil || errors.Is(err, experience.ErrForbidden) {
		t.Fatalf("database cancellation became permission denial: allowed=%v error=%v", allowed, err)
	}
}
