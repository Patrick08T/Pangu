package catalog

import (
	"github.com/Patrick08T/Pangu/internal/workflow"
	"reflect"
)

// ValidateInstructionCandidate 只允许自动候选修改角色指令。
func ValidateInstructionCandidate(base, candidate workflow.Node) error {
	if err := candidate.Validate(); err != nil {
		return invalid(err.Error())
	}
	base.Instruction, candidate.Instruction = "", ""
	if !reflect.DeepEqual(base, candidate) {
		return invalid("自动候选只能修改 Instruction")
	}
	return nil
}
