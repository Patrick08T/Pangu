package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

type Backend interface {
	Submit(context.Context, workflow.Definition) (string, error)
	GetRun(context.Context, string) (*scheduler.RunDetails, error)
	CreateCron(context.Context, workflow.Cron) (*rest.CronWorkflows, error)
	ListCrons(context.Context, int64) (*rest.CronWorkflowsList, error)
	DeleteCron(context.Context, string) error
}

type handler struct{ backend Backend }

func New(backend Backend) http.Handler {
	h := handler{backend: backend}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /runs", h.submit)
	mux.HandleFunc("GET /runs/{id}", h.getRun)
	mux.HandleFunc("POST /crons", h.createCron)
	mux.HandleFunc("GET /crons", h.listCrons)
	mux.HandleFunc("DELETE /crons/{id}", h.deleteCron)
	return http.TimeoutHandler(mux, 30*time.Second, `{"error":"请求超时"}`)
}

func (h handler) submit(w http.ResponseWriter, r *http.Request) {
	var d workflow.Definition
	if !decode(w, r, &d) {
		return
	}
	if _, err := d.Layers(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	id, err := h.backend.Submit(r.Context(), d)
	if backendError(w, err) {
		return
	}
	w.Header().Set("Location", "/runs/"+id)
	write(w, 202, map[string]string{"run_id": id})
}

func (h handler) getRun(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	out, err := h.backend.GetRun(r.Context(), r.PathValue("id"))
	if !backendError(w, err) {
		write(w, 200, out)
	}
}

func (h handler) createCron(w http.ResponseWriter, r *http.Request) {
	var c workflow.Cron
	if !decode(w, r, &c) {
		return
	}
	if err := c.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	out, err := h.backend.CreateCron(r.Context(), c)
	if !backendError(w, err) {
		write(w, 201, out)
	}
}

func (h handler) listCrons(w http.ResponseWriter, r *http.Request) {
	var offset int64
	if value := r.URL.Query().Get("offset"); value != "" {
		var err error
		offset, err = strconv.ParseInt(value, 10, 64)
		if err != nil || offset < 0 {
			problem(w, 400, "offset 必须是非负整数")
			return
		}
	}
	out, err := h.backend.ListCrons(r.Context(), offset)
	if !backendError(w, err) {
		write(w, 200, out)
	}
}

func (h handler) deleteCron(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !backendError(w, h.backend.DeleteCron(r.Context(), r.PathValue("id"))) {
		w.WriteHeader(204)
	}
}

func decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		problem(w, 415, "Content-Type 必须为 application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(dest)
	if err == nil {
		var extra any
		if next := decoder.Decode(&extra); next != io.EOF {
			err = next
			if err == nil {
				err = fmt.Errorf("仅允许一个 JSON 对象")
			}
		}
	}
	if err == nil {
		return true
	}
	var sizeError *http.MaxBytesError
	if errors.As(err, &sizeError) {
		problem(w, 413, "请求体超过 256 KiB")
	} else {
		problem(w, 400, "无效 JSON: "+err.Error())
	}
	return false
}

func validID(w http.ResponseWriter, r *http.Request) bool {
	if _, err := uuid.Parse(r.PathValue("id")); err != nil {
		problem(w, 400, "ID 必须为 UUID")
		return false
	}
	return true
}

func backendError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, scheduler.ErrNotFound) {
		problem(w, 404, err.Error())
		return true
	}
	if errors.Is(err, scheduler.ErrNotReady) {
		problem(w, 503, err.Error())
		return true
	}
	problem(w, 502, "Hatchet 请求失败，请检查服务连接、资源 ID 和服务端日志")
	return true
}

func problem(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]string{"error": message})
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
