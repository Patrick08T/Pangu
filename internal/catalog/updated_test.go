package catalog

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func objectUpdated(t *testing.T, item *Object) time.Time {
	t.Helper()
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var fields struct {
		Updated time.Time `json:"updated_at"`
	}
	if err = json.Unmarshal(raw, &fields); err != nil || fields.Updated.IsZero() {
		t.Fatalf("配置未返回数据库更新时间：%s，%v", raw, err)
	}
	return fields.Updated
}

func TestCatalogFiltersBeforePaginationAndOrdersByUpdate(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec,archived,updated_at)
		SELECT gen_random_uuid(),$1,'workflow','旧测试'||i,'{"flow":{"nodes":[]}}',true,now()
		FROM generate_series(1,101) i`, ns)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec,updated_at)
		VALUES (gen_random_uuid(),$1,'workflow','较新流程','{"flow":{"nodes":[]}}',now()-interval '1 day'),
		(gen_random_uuid(),$1,'workflow','较早流程','{"flow":{"nodes":[]}}',now()-interval '2 days')`, ns)
	if err != nil {
		t.Fatal(err)
	}
	archived := false
	active, err := List(ctx, ns, "workflow", 0, &archived)
	if err != nil || len(active) != 2 || active[0].Name != "较新流程" || active[1].Name != "较早流程" {
		t.Fatalf("归档项挤占分页或更新排序错误：%+v %v", active, err)
	}
	if objectUpdated(t, &active[0]).IsZero() {
		t.Fatal("列表未返回更新时间")
	}
	archived = true
	old, err := List(ctx, ns, "workflow", 100, &archived)
	if err != nil || len(old) != 1 || !old[0].Archived {
		t.Fatalf("归档分页错误：%+v %v", old, err)
	}
	all, err := List(ctx, ns, "workflow", 100, nil)
	if err != nil || len(all) != 3 {
		t.Fatalf("全部筛选错误：%+v %v", all, err)
	}
}

func TestCatalogUpdatedAtTracksSavePublishAndArchive(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	ctx := context.Background()
	item, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "更新时间", Spec: Spec{Agent: &workflow.Node{ID: "one", Executor: "codex", Handoff: testHandoff(), Instruction: "检查"}}})
	if err != nil {
		t.Fatal(err)
	}
	created := objectUpdated(t, item)
	if !created.Equal(item.CreatedAt) {
		t.Fatal("新配置时间不一致")
	}
	saved, err := Save(ctx, ns, item.ID, item.Revision, "更新名称", item.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if !objectUpdated(t, saved).After(created) {
		t.Fatal("保存没有刷新更新时间")
	}
	if _, err = Publish(ctx, ns, saved.ID, saved.Revision); err != nil {
		t.Fatal(err)
	}
	published, err := Get(ctx, ns, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	stamp := objectUpdated(t, published)
	if !stamp.After(objectUpdated(t, saved)) {
		t.Fatal("发布没有刷新更新时间")
	}
	if _, err = Publish(ctx, ns, saved.ID, saved.Revision); err != nil {
		t.Fatal(err)
	}
	repeated, err := Get(ctx, ns, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !objectUpdated(t, repeated).Equal(stamp) {
		t.Fatal("重复发布不应制造更新时间")
	}
	if err = Archive(ctx, ns, item.ID, true); err != nil {
		t.Fatal(err)
	}
	archived, err := Get(ctx, ns, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !objectUpdated(t, archived).After(stamp) || !archived.CreatedAt.Equal(item.CreatedAt) {
		t.Fatal("归档更新不应覆盖创建时间")
	}
}
