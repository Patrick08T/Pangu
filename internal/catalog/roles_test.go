package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestCollaborationRoleTemplates(t *testing.T) {
	roles := map[string]bool{
		"dev-engineer": false, "qa-engineer": false, "pm-planner": false,
		"acceptance-reviewer": true, "game-performance-engineer": false,
		"game-security-engineer": true, "game-release-engineer": true,
	}
	for name, readOnly := range roles {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "templates", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var item struct {
				Format string `json:"format"`
				NewObject
			}
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&item); err != nil {
				t.Fatal(err)
			}
			if err := item.Validate(); err != nil {
				t.Fatal(err)
			}
			node := item.Spec.Agent
			if item.Format != "pangu.catalog.v1" || item.Kind != "agent" || node.ID != name {
				t.Fatal("角色必须保留标识并在工作空间共享")
			}
			if node.Executor != "codex" || node.ReadOnly != readOnly || node.ConnectionVersion != "" || node.EffectivePolicy().Retries != 0 {
				t.Fatal("角色执行权限或账号隔离不正确")
			}
			if err := node.Handoff.Validate(true); err != nil {
				t.Fatal(err)
			}
			checkGeneralRole(t, node)
			definition := workflow.Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Nodes: []workflow.Node{*node}}
			if _, err := definition.Layers(); err != nil {
				t.Fatal(err)
			}
			for _, stale := range []string{"/workspace/taotaopeng", "pending CL", "指定 CL", "Partopia 默认忽略"} {
				if strings.Contains(node.Instruction, stale) {
					t.Fatalf("残留源项目约定：%s", stale)
				}
			}
		})
	}
}

func checkGeneralRole(t *testing.T, node *workflow.Node) {
	t.Helper()
	if node.Model != "" || node.Role != "" || node.OutputSchema != nil || node.Approval != nil || node.EffectivePolicy() != (workflow.Policy{Retries: 0}) {
		t.Fatal("通用化不得改变模型、输出协议、审批或预算")
	}
	text := node.Description + node.Instruction + node.Handoff.Instruction()
	for _, projectRule := range []string{"Pangu", "ONEPIECE", "Partopia", "P4", "http://10.", "Go 用", "build + vet", "golangci", "-race", "proto/csv", "Up/Dn", "机器人连联调网关"} {
		if strings.Contains(text, projectRule) {
			t.Errorf("通用职责不应绑定项目或工具：%s", projectRule)
		}
	}
}
