package catalog

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func candidateBase() workflow.Node {
	return workflow.Node{ID: "agent", Executor: "codex", Instruction: "检查项目", Handoff: testHandoff()}
}

func TestInstructionCandidateProtectsEveryOtherField(t *testing.T) {
	base := candidateBase()
	fields := reflect.TypeOf(base)
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		if field.Name == "Instruction" {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			changed := base
			value := reflect.ValueOf(&changed).Elem().Field(i)
			if value.IsZero() {
				makeNonzero(value)
			} else {
				value.SetZero()
			}
			if err := ValidateInstructionCandidate(base, changed); !errors.Is(err, ErrInvalid) {
				t.Fatalf("允许修改受保护字段 %s: %v", field.Name, err)
			}
		})
	}
}

func makeNonzero(value reflect.Value) {
	switch value.Kind() {
	case reflect.String:
		value.SetString("changed")
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
	case reflect.Map:
		value.Set(reflect.MakeMap(value.Type()))
	default:
		panic("新增字段需要提供非零测试值: " + value.Type().String())
	}
}

func TestInstructionCandidateAllowsOnlyValidInstruction(t *testing.T) {
	base := candidateBase()
	changed := base
	changed.Instruction = "先验证证据，再检查项目"
	if err := ValidateInstructionCandidate(base, changed); err != nil {
		t.Fatal(err)
	}
	if base.Instruction != "检查项目" {
		t.Fatal("候选修改了基线")
	}
	changed.Instruction = "  "
	if err := ValidateInstructionCandidate(base, changed); !errors.Is(err, ErrInvalid) {
		t.Fatalf("接受空指令: %v", err)
	}
}
