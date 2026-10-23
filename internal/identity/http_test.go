package identity

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func httpIdentity(t *testing.T) (Service, Actor, string) {
	t.Helper()
	ctx, ns := identityDB(t)
	for key, value := range map[string]string{"PANGU_FEISHU_APP_ID": "cli_test", "PANGU_FEISHU_APP_SECRET": "secret", "PANGU_PUBLIC_URL": "https://pangu.example", "PANGU_FEISHU_TENANT_KEY": "tenant", "PANGU_FEISHU_ADMIN_OPEN_ID": "ou_admin"} {
		t.Setenv(key, value)
	}
	cfg, err := config()
	if err != nil {
		t.Fatal(err)
	}
	actor, token, err := login(ctx, ns, cfg, FeishuUser{OpenID: cfg.AdminOpenID, Tenant: cfg.Tenant})
	if err != nil {
		t.Fatal(err)
	}
	return Service{Namespace: ns}, actor, token
}

func TestHTTPIdentityRejectsAmbiguityAndCSRF(t *testing.T) {
	service, actor, token := httpIdentity(t)
	h := service.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if Current(r.Context()).ID != actor.ID {
			t.Error("缺少身份上下文")
		}
		w.WriteHeader(204)
	}))
	for _, tc := range []struct {
		name, method, origin, csrf, authorization string
		cookies                                   int
		want                                      int
	}{
		{"anonymous", "GET", "", "", "", 0, 401},
		{"session", "GET", "", "", "", 1, 204},
		{"duplicate cookie", "GET", "", "", "", 2, 401},
		{"ambiguous", "GET", "", "", "Bearer pgu_bad", 1, 401},
		{"missing csrf", "POST", "https://pangu.example", "", "", 1, 403},
		{"foreign origin", "POST", "https://evil.example", actor.CSRF, "", 1, 403},
		{"missing origin", "POST", "", actor.CSRF, "", 1, 403},
		{"valid mutation", "POST", "https://pangu.example", actor.CSRF, "", 1, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/runs", nil)
			for i := 0; i < tc.cookies; i++ {
				r.AddCookie(&http.Cookie{Name: "pangu_session", Value: token})
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.csrf != "" {
				r.Header.Set("X-CSRF-Token", tc.csrf)
			}
			if tc.authorization != "" {
				r.Header.Set("Authorization", tc.authorization)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestLoginHTTPOnlyHealthIsPublicWhenUnconfigured(t *testing.T) {
	t.Setenv("PANGU_FEISHU_APP_ID", "")
	h := (Service{Namespace: "test"}).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for path, want := range map[string]int{"/healthz": 204, "/readyz": 503, "/auth/login": 503, "/auth/me": 503} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
}
