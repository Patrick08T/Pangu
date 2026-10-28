package factory

import (
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"testing"
)

func TestReplyRejectsMalformedAndUnrequestedChanges(t *testing.T) {
	for _, raw := range []string{`{"message":"ok","execute":true}`, `{"message":"ok"} {}`, "```json\n{}\n```", `{"message":""}`} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("接受无效回复 %s", raw)
		}
	}
	r, err := Parse(`{"message":"还需要提供目标用户"}`)
	if err != nil || r.Proposal != nil {
		t.Fatal("澄清回复解析失败", err)
	}
	b := catalog.Bundle{Agents: []catalog.AgentChange{{ObjectID: uuid.NewString()}}}
	if err := checkTargets(b, Input{}); err == nil {
		t.Fatal("模型可以修改未指定对象")
	}
}

func TestGeneratorIsRestrictedAndLoadsBuiltinSkill(t *testing.T) {
	s := Session{ProjectID: uuid.NewString()}
	in := Input{ID: uuid.NewString(), Revision: 1, Message: "创建一个产品经理", Mode: "build", ConnectionVersion: uuid.NewString()}
	d, digest, err := Definition(s, in, Context{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Layers(); err != nil {
		t.Fatal(err)
	}
	if err = d.ValidateHandoffs(); err != nil {
		t.Fatal(err)
	}
	n := d.Nodes[0]
	if n.Executor != "eino" || n.Role != "" || n.Policy.Retries != 0 || d.Budget.CodexSessions != 1 || len(digest) != 64 {
		t.Fatal("生成助手扩大执行权限")
	}
	if len(d.Input) < 10000 || len(n.Instruction) > 16384 {
		t.Fatal("技能没有接入或指令超长")
	}
	_ = workflow.Budget{}
}
