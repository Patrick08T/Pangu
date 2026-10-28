package api

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/google/uuid"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestAgentTrialConcurrentAcceptanceAndChangedDraft(t *testing.T) {
	item, project := trialObject(t)
	ctx := context.Background()
	d, err := catalog.CompileDraft(ctx, "api-test", item.ID, 1, project, "输入")
	if err != nil {
		t.Fatal(err)
	}
	key := trialKey()
	results := make(chan string, 4)
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			record, err := submission.PrepareTrial(ctx, "api-test", item.ID, 1, *d, key)
			if err != nil {
				t.Error(err)
				return
			}
			results <- record.ID
		})
	}
	group.Wait()
	close(results)
	id := ""
	for result := range results {
		if id != "" && id != result {
			t.Fatal("创建了重复意图")
		}
		id = result
	}
	if id == "" {
		t.Fatal("没有提交意图")
	}
	item.Spec.Agent.Instruction = "已更新"
	if _, err = catalog.Save(ctx, "api-test", item.ID, 1, item.Name, item.Spec); err != nil {
		t.Fatal(err)
	}
	if _, err = submission.PrepareTrial(ctx, "api-test", item.ID, 1, *d, trialKey()); !errors.Is(err, submission.ErrConflict) {
		t.Fatal("旧修订被接受", err)
	}
	rows, err := submission.Trials(ctx, "api-test", item.ID, project, 0)
	if err != nil || len(rows) != 1 {
		t.Fatal("关联数量错误", len(rows), err)
	}
}

func TestAgentTrialExpiredKeyStillRejectedAfterReceiptDeletion(t *testing.T) {
	item, project := trialObject(t)
	ctx := context.Background()
	d, err := catalog.CompileDraft(ctx, "api-test", item.ID, 1, project, "过期输入")
	if err != nil {
		t.Fatal(err)
	}
	key := strconv.FormatInt(time.Now().Add(-91*24*time.Hour).UnixMilli(), 10) + "." + uuid.NewString()
	record, err := submission.Prepare(ctx, "api-test", "submit", submission.TrialKey(key), submission.Snapshot{Workflow: *d})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "INSERT INTO pangu.agent_trials(request_id,object_id,revision) VALUES($1,$2,1)", record.ID, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, "DELETE FROM pangu.submissions WHERE id=$1", record.ID); err != nil {
		t.Fatal(err)
	}
	response := trialRequest(New(&trialBackend{}, testAccess{}), item.ID, project, "POST", "/trials", key, `{"revision":1,"input":"过期输入"}`)
	if response.Code != 409 {
		t.Fatalf("过期键重新派发: %d %s", response.Code, response.Body.String())
	}
	rows, err := submission.Trials(ctx, "api-test", item.ID, project, 0)
	if err != nil || len(rows) != 0 {
		t.Fatal("旧键产生了新意图", err)
	}
}
