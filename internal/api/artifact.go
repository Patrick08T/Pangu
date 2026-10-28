package api

import (
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"mime"
	"net/http"
	"path"

	"github.com/Patrick08T/Pangu/internal/artifact"
)

func (h handler) artifactRun(w http.ResponseWriter, r *http.Request) bool {
	if !validID(w, r) {
		return false
	}
	if len(r.URL.Query()) != 0 {
		problem(w, 400, "产物查询不接受路径或其他查询参数")
		return false
	}
	return h.runAllowed(w, r, "read")
}

func (h handler) artifacts(w http.ResponseWriter, r *http.Request) {
	if !h.artifactRun(w, r) {
		return
	}
	out, err := artifact.List(r.Context(), r.PathValue("id"))
	if !artifactError(w, err) {
		write(w, 200, out)
	}
}

func (h handler) artifact(w http.ResponseWriter, r *http.Request) {
	if !h.artifactRun(w, r) {
		return
	}
	id := r.PathValue("artifact")
	_, uuidErr := uuid.Parse(id)
	if value, err := hex.DecodeString(id); (err != nil || len(value) != 16) && uuidErr != nil {
		problem(w, 400, "产物 ID 无效")
		return
	}
	diff := r.Pattern == "GET /runs/{id}/artifacts/{artifact}/diff"
	file, data, truncated, err := artifact.Read(r.Context(), r.PathValue("id"), id, diff)
	if artifactError(w, err) {
		return
	}
	if diff {
		write(w, 200, map[string]any{"file": file, "diff": string(data), "truncated": truncated})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if !file.Binary {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(file.Path)}))
	_, _ = w.Write(data)
}

func (h handler) exportArtifacts(w http.ResponseWriter, r *http.Request) {
	if !h.artifactRun(w, r) {
		return
	}
	data, err := artifact.Export(r.Context(), r.PathValue("id"))
	if artifactError(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="pangu-artifacts.zip"`)
	_, _ = w.Write(data)
}

func artifactError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, artifact.ErrNotFound) {
		problem(w, 404, err.Error())
		return true
	}
	if errors.Is(err, artifact.ErrBlocked) {
		problem(w, 409, err.Error())
		return true
	}
	return backendError(w, err)
}
