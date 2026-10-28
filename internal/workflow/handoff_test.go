package workflow

import (
	"strings"
	"testing"
)

func TestHandoffRequiredFieldsAndDraftLimits(t *testing.T) {
	var missing *HandoffContract
	if missing.Validate(false) != nil || missing.Validate(true) == nil {
		t.Fatal("空契约的草稿与执行边界错误")
	}
	h := HandoffContract{Objective: "完成任务", Inputs: "本次输入", Deliverables: "报告", Acceptance: "有证据", FailureHandling: "说明缺口"}
	for _, field := range []*string{&h.Objective, &h.Inputs, &h.Deliverables, &h.Acceptance, &h.FailureHandling} {
		original := *field
		*field = " \t\n　"
		if h.Validate(true) == nil || h.Validate(false) != nil {
			t.Fatal("空白必填项未被拒绝或不能暂存")
		}
		for _, bad := range []string{strings.Repeat("中", 1366), string([]byte{0xff})} {
			*field = bad
			if h.Validate(false) == nil {
				t.Fatal("草稿接受超限或非法文本")
			}
		}
		*field = original
	}
	if h.Validate(true) != nil || !strings.Contains(h.Instruction(), "不得扩大本次资料授权") {
		t.Fatal("有效契约未保留权限边界")
	}
}
