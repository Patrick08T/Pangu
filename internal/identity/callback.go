package identity

import (
	"log/slog"
	"net/http"
	"strings"
)

func setCookie(w http.ResponseWriter, cfg Config, name, value string, seconds int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/api", HttpOnly: true, Secure: strings.HasPrefix(cfg.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: seconds})
}

func (s Service) start(w http.ResponseWriter, r *http.Request, cfg Config) {
	if r.URL.RawQuery != "" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		authFailure(w, ErrForbidden)
		return
	}
	location, browser, err := beginLogin(r.Context(), s.Namespace, cfg)
	if err != nil {
		authFailure(w, err)
		return
	}
	setCookie(w, cfg, "pangu_login", browser, 300)
	http.Redirect(w, r, location, http.StatusFound)
}

func (s Service) callback(w http.ResponseWriter, r *http.Request, cfg Config) {
	q := r.URL.Query()
	cookies := r.CookiesNamed("pangu_login")
	if len(cookies) != 1 || len(q["state"]) != 1 || len(q["code"]) > 1 || len(q["error"]) > 1 {
		authFailure(w, ErrUnauthenticated)
		return
	}
	for key := range q {
		if key != "state" && key != "code" && key != "error" && key != "error_description" {
			authFailure(w, ErrUnauthenticated)
			return
		}
	}
	setCookie(w, cfg, "pangu_login", "", -1)
	err := consumeLogin(r.Context(), s.Namespace, cfg, q.Get("state"), cookies[0].Value)
	if err != nil {
		authFailure(w, err)
		return
	}
	if q.Get("error") != "" || len(q["code"]) != 1 || len(q.Get("code")) > 2048 || q.Get("code") == "" {
		authFailure(w, ErrUnauthenticated)
		return
	}
	external, err := exchange(r.Context(), feishuClient(), cfg, q.Get("code"))
	if err != nil {
		slog.ErrorContext(r.Context(), "飞书身份交换失败", "error", err)
		authFailure(w, err)
		return
	}
	_, token, err := login(r.Context(), s.Namespace, cfg, external)
	if err != nil {
		authFailure(w, err)
		return
	}
	if old := r.CookiesNamed("pangu_session"); len(old) == 1 {
		if err = logout(r.Context(), s.Namespace, old[0].Value); err != nil {
			authFailure(w, err)
			return
		}
	}
	setCookie(w, cfg, "pangu_session", token, 12*60*60)
	http.Redirect(w, r, cfg.PublicURL+"/", http.StatusSeeOther)
}

func (s Service) end(w http.ResponseWriter, r *http.Request, cfg Config, actor Actor) {
	if actor.TokenID != "" {
		authFailure(w, ErrForbidden)
		return
	}
	cookie, err := r.Cookie("pangu_session")
	if err != nil {
		authFailure(w, ErrUnauthenticated)
		return
	}
	if err = logout(r.Context(), s.Namespace, cookie.Value); err != nil {
		authFailure(w, err)
		return
	}
	setCookie(w, cfg, "pangu_session", "", -1)
	w.WriteHeader(http.StatusNoContent)
}
