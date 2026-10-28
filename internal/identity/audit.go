package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/audit"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func audited(next http.Handler, w http.ResponseWriter, r *http.Request, actor Actor) {
	if r.Method == "GET" || r.Method == "HEAD" || r.Pattern == "" || r.URL.Path == "/workflows/validate" || r.URL.Path == "/workflows/preflight" {
		next.ServeHTTP(w, r)
		return
	}
	id := r.Header.Get("X-Request-ID")
	if _, err := uuid.Parse(id); err != nil {
		id = uuid.NewString()
	}
	entry := audit.Entry{ID: uuid.NewString(), CreatedAt: time.Now().UTC(), Namespace: actor.Namespace, ActorID: actor.ID, TokenID: actor.TokenID, RequestID: id, Action: r.Pattern}
	for _, part := range strings.Split(r.URL.Path, "/") {
		if _, err := uuid.Parse(part); err == nil {
			if entry.ObjectID == "" {
				entry.ObjectID = part
			} else {
				entry.TargetID = part
			}
		}
	}
	event, err := audit.Begin(r.Context(), entry)
	if err != nil && r.Pattern != "POST /runs/{id}/cancel" {
		authError(w, 503, "无法记录操作审计，操作未执行")
		return
	}
	if err != nil {
		event = &entry
		queueAudit(entry, 0)
	}
	if event != nil {
		w.Header().Set("X-Audit-ID", event.ID)
		r = r.WithContext(audit.WithEntry(r.Context(), event))
	}
	response := &auditWriter{ResponseWriter: w, status: 200}
	next.ServeHTTP(response, r)
	if event == nil {
		return
	}
	response.result(event)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = audit.Finish(ctx, event, response.status); err != nil {
		queueAudit(*event, response.status)
	}
}

func queueAudit(entry audit.Entry, status int) {
	if err := audit.Queue(entry, status); err != nil {
		slog.Error("数据库与本地审计补录均失败，需人工核对", "audit_id", entry.ID, "request_id", entry.RequestID, "actor_id", entry.ActorID, "run_id", entry.ObjectID, "http_status", status)
	} else {
		slog.Warn("审计已持久暂存，将自动补录", "audit_id", entry.ID, "request_id", entry.RequestID)
	}
}

type auditWriter struct {
	http.ResponseWriter
	status   int
	wrote    bool
	body     bytes.Buffer
	overflow bool
}

func (w *auditWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	if code >= 200 {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *auditWriter) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(200)
	}
	if w.body.Len()+len(data) <= 64*1024 && !w.overflow {
		_, _ = w.body.Write(data)
	} else {
		w.body.Reset()
		w.overflow = true
	}
	return w.ResponseWriter.Write(data)
}
func (w *auditWriter) result(event *audit.Entry) {
	if w.status >= 400 || w.overflow {
		return
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(w.body.Bytes(), &value) != nil {
		return
	}
	for _, key := range []string{"request_id", "id", "run_id"} {
		var id string
		if json.Unmarshal(value[key], &id) == nil {
			if _, err := uuid.Parse(id); err == nil {
				event.ResultID = id
				break
			}
		}
	}
	var version int
	for _, key := range []string{"number", "revision"} {
		if json.Unmarshal(value[key], &version) == nil && version > 0 {
			event.ResultVersion = &version
			break
		}
	}
}
