package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func observe(mux *http.ServeMux, access Access) http.Handler {
	next := http.TimeoutHandler(access.Handler(mux), 30*time.Second, `{"error":"请求超时"}`)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, started := uuid.NewString(), time.Now()
		_, pattern := mux.Handler(r)
		r.Pattern = pattern
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("Cache-Control", "no-store")
		response := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(response, r)
		// 只记录路由模板，避免路径、查询参数、请求体中的用户内容进入访问日志。
		slog.InfoContext(r.Context(), "HTTP 请求", "request_id", id, "route", pattern,
			"status", response.status, "duration_ms", time.Since(started).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	if status >= 200 {
		w.status, w.wrote = status, true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
