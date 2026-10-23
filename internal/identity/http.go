package identity

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Service struct{ Namespace string }

func (s Service) Authorize(ctx context.Context, project, action, owner string) error {
	return Authorize(ctx, Current(ctx), project, action, owner)
}

func (s Service) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == "GET" && r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		cfg, err := config()
		if err != nil {
			authError(w, 503, "飞书身份服务尚未配置，请联系管理员")
			return
		}
		if r.Method == "GET" && r.URL.Path == "/auth/login" {
			s.start(w, r, cfg)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/auth/callback" {
			s.callback(w, r, cfg)
			return
		}
		actor, err := s.authenticate(r, cfg)
		if err != nil {
			authFailure(w, err)
			return
		}
		if actor.TokenID == "" && r.Method != "GET" && r.Method != "HEAD" {
			if !sameOrigin(r, cfg) || len(r.Header.Values("X-CSRF-Token")) != 1 || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(actor.CSRF)) != 1 {
				authFailure(w, ErrForbidden)
				return
			}
		}
		r = r.WithContext(WithActor(r.Context(), actor))
		if r.Method == "GET" && r.URL.Path == "/auth/me" {
			authJSON(w, 200, actor)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/auth/logout" {
			s.end(w, r, cfg, actor)
			return
		}
		audited(next, w, r, actor)
	})
}

func (s Service) authenticate(r *http.Request, cfg Config) (Actor, error) {
	cookies := r.CookiesNamed("pangu_session")
	headers := r.Header.Values("Authorization")
	if len(cookies) > 1 || len(headers) > 1 || len(cookies)+len(headers) != 1 {
		return Actor{}, ErrUnauthenticated
	}
	if len(headers) == 1 {
		if !strings.HasPrefix(headers[0], "Bearer ") {
			return Actor{}, ErrUnauthenticated
		}
		if len(r.Header.Values("Origin")) > 0 && !sameOrigin(r, cfg) {
			return Actor{}, ErrForbidden
		}
		return tokenActor(r.Context(), s.Namespace, cfg, strings.TrimPrefix(headers[0], "Bearer "))
	}
	return sessionActor(r.Context(), s.Namespace, cfg, cookies[0].Value)
}

func sameOrigin(r *http.Request, cfg Config) bool {
	return len(r.Header.Values("Origin")) == 1 && r.Header.Get("Origin") == cfg.PublicURL
}

func authJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func authError(w http.ResponseWriter, status int, message string) {
	authJSON(w, status, map[string]string{"error": message})
}
func authFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		authError(w, 401, ErrUnauthenticated.Error())
	case errors.Is(err, ErrForbidden):
		authError(w, 403, ErrForbidden.Error())
	default:
		authError(w, 503, "身份服务暂不可用，请稍后重试")
	}
}
