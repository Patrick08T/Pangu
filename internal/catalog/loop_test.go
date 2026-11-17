package catalog

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestLoopBundlePublishesAndCompilesWithoutLosingControl(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	b := bundleFixture()
	b.Agents[0].Node.OutputSchema = workflow.LoopGateSchema()
	data, _ := json.Marshal(b)
	data = []byte(strings.TrimSuffix(string(data), "}") + `,"loops":[{"id":"repair","nodes":["first","second"],"gates":["second"]}]}`)
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	out, err := ApplyBundleTx(context.Background(), tx, ns, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	d, err := Compile(context.Background(), ns, out.Workflow.ID, project, "修复")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Loops) != 1 || d.Loops[0].ID != "repair" {
		t.Fatal("发布/编译丢失循环定义")
	}
	if err = CheckDefinition(context.Background(), ns, *d); err != nil {
		t.Fatal(err)
	}
	d.Loops = nil
	if err = CheckDefinition(context.Background(), ns, *d); err == nil {
		t.Fatal("删除循环仍冒充原版本")
	}
}

func TestBugfixLoopExampleIsPublishable(t *testing.T) {
	ns, _, _ := catalogDatabase(t)
	data, err := os.ReadFile("../../examples/workflows/bugfix-loop.json")
	if err != nil {
		t.Fatal(err)
	}
	var b Bundle
	if err = json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Agents) != 6 || len(b.Loops) != 1 || len(b.Loops[0].Gates) != 3 {
		t.Fatal("示例缺少六角色三质量门")
	}
	if err = CheckBundle(context.Background(), ns, b); err != nil {
		t.Fatal(err)
	}
}
