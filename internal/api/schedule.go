package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (h handler) getCron(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	item, err := h.backend.GetCron(r.Context(), r.PathValue("id"))
	if !backendError(w, err) && h.allowed(w, r, item.Workflow.ProjectID, "read", item.CreatedBy) {
		write(w, 200, item)
	}
}

func (h handler) cronPreview(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) != 1 || len(r.URL.Query()["expression"]) != 1 {
		problem(w, 400, "仅接受 expression 参数")
		return
	}
	times, err := workflow.Preview(r.URL.Query().Get("expression"), time.Now())
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	write(w, 200, times)
}

func (h handler) editCron(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if !h.cronAllowed(w, r, "manage_schedule") {
		return
	}
	var input struct {
		Revision int           `json:"revision"`
		Paused   bool          `json:"paused"`
		Cron     workflow.Cron `json:"cron"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Cron.Workflow.VersionID != "" {
		c, err := h.resolveCron(r.Context(), r.PathValue("id"), input.Cron, input.Paused)
		if backendError(w, err) {
			return
		}
		input.Cron = c
	}
	if err := input.Cron.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	item, err := h.backend.UpdateCron(r.Context(), r.PathValue("id"), input.Revision, input.Cron, input.Paused)
	if !backendError(w, err) {
		write(w, 200, item)
	}
}

func (h handler) resolveCron(ctx context.Context, id string, c workflow.Cron, paused bool) (workflow.Cron, error) {
	if paused {
		plan, err := schedule.Get(ctx, h.backend.Namespace(), id)
		if err != nil {
			return c, err
		}
		if c.Workflow.VersionID == plan.Workflow.VersionID && c.Workflow.Input == plan.Workflow.Input {
			if len(c.Workflow.Nodes) > 0 && !reflect.DeepEqual(c.Workflow, plan.Workflow) {
				return c, fmt.Errorf("%w: 暂停不能改写已有版本快照", catalog.ErrInvalid)
			}
			c.Workflow = plan.Workflow
			return c, nil
		}
	}
	d, err := catalog.Compile(ctx, h.backend.Namespace(), c.Workflow.VersionID, c.Workflow.ProjectID, c.Workflow.Input)
	if err != nil {
		return c, err
	}
	if len(c.Workflow.Nodes) > 0 {
		if err := catalog.CheckDefinition(ctx, h.backend.Namespace(), c.Workflow); err != nil {
			return c, err
		}
	}
	if c.Workflow.ProjectID != "" && c.Workflow.ProjectID != d.ProjectID {
		return c, fmt.Errorf("%w: 版本不属于指定项目", catalog.ErrInvalid)
	}
	c.Workflow = *d
	return c, nil
}

func (h handler) cronFires(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	offset := int64(0)
	for key, values := range r.URL.Query() {
		if key != "offset" || len(values) != 1 {
			problem(w, 400, "只接受一个 offset 参数")
			return
		}
		var err error
		offset, err = strconv.ParseInt(values[0], 10, 64)
		if err != nil || offset < 0 {
			problem(w, 400, "offset 必须非负")
			return
		}
	}
	if !h.cronAllowed(w, r, "read") {
		return
	}
	out, err := h.backend.CronFires(r.Context(), r.PathValue("id"), offset)
	if !backendError(w, err) {
		write(w, 200, out)
	}
}
